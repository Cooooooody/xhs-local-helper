param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path,
  [string]$Version = $(if ($env:WINDOWS_MSI_VERSION) { $env:WINDOWS_MSI_VERSION } else { "0.1.0" })
)

$ErrorActionPreference = "Stop"

$distDir = Join-Path $RepoRoot "dist\windows-x64"
$bundleDir = Join-Path $distDir "xhs-local-helper-windows-x64"
$installerDir = Join-Path $distDir "installer"
$msiPath = Join-Path $distDir "xhs-local-helper-windows-x64.msi"
$goBin = if ($env:GO_BIN) { $env:GO_BIN } else { "go" }
$wixExe = Get-Command wix -ErrorAction SilentlyContinue

if (-not (Test-Path $bundleDir)) {
  throw "missing Windows bundle directory: $bundleDir`nRun scripts/windows/package-bundle.sh first."
}

foreach ($required in @(
  "xhs-local-helper-windows-amd64.exe",
  "xhs-local-helper-windows-tray-amd64.exe",
  "xiaohongshu-mcp-windows-amd64.zip",
  "start-helper.bat",
  "stop-helper.bat"
)) {
  $path = Join-Path $bundleDir $required
  if (-not (Test-Path $path)) {
    throw "missing required bundle payload: $path"
  }
}

if (-not $wixExe) {
  throw "missing WiX CLI 'wix' in PATH"
}

New-Item -ItemType Directory -Force -Path $installerDir | Out-Null
Remove-Item $msiPath -Force -ErrorAction SilentlyContinue

& $goBin run ./cmd/generate-windows-msi-assets $RepoRoot $installerDir $Version
if ($LASTEXITCODE -ne 0) {
  throw "failed to generate MSI assets"
}

& $wixExe.Source build (Join-Path $installerDir "Product.wxs") `
  -ext WixToolset.Util.wixext `
  -arch x64 `
  -o $msiPath
if ($LASTEXITCODE -ne 0) {
  throw "wix build failed"
}

Write-Host "Created Windows MSI: $msiPath"
