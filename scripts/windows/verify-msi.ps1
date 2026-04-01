param(
  [string]$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
)

$ErrorActionPreference = "Stop"

$distDir = Join-Path $RepoRoot "dist\windows-x64"
$installerDir = Join-Path $distDir "installer"
$msiPath = Join-Path $distDir "xhs-local-helper-windows-x64.msi"
$wxsPath = Join-Path $installerDir "Product.wxs"

if (-not (Test-Path $msiPath)) {
  throw "missing MSI artifact: $msiPath"
}
if (-not (Test-Path $wxsPath)) {
  throw "missing WiX source: $wxsPath"
}

$wxs = Get-Content $wxsPath -Raw
foreach ($needle in @(
  'xhs-local-helper-windows-amd64.exe',
  'xhs-local-helper-windows-tray-amd64.exe',
  'xiaohongshu-mcp-windows-amd64.zip',
  'start-helper.bat',
  'stop-helper.bat',
  'ProgramMenuFolder',
  'Shortcut',
  'util:CloseApplication'
)) {
  if (-not $wxs.Contains($needle)) {
    throw "WiX source is missing expected content: $needle"
  }
}

Write-Host "Windows MSI verification passed: $msiPath"
