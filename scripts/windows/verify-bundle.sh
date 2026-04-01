#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
BUNDLE_DIR="${1:-$ROOT_DIR/dist/windows-x64/xhs-local-helper-windows-x64}"
ZIP_PATH="${2:-$ROOT_DIR/dist/windows-x64/xhs-local-helper-windows-x64.zip}"

test -d "$BUNDLE_DIR"
test -f "$BUNDLE_DIR/xhs-local-helper-windows-amd64.exe"
test -f "$BUNDLE_DIR/xhs-local-helper-windows-tray-amd64.exe"
test -f "$BUNDLE_DIR/xiaohongshu-mcp-windows-amd64.zip"
test -f "$BUNDLE_DIR/start-helper.bat"
test -f "$BUNDLE_DIR/stop-helper.bat"
test -f "$BUNDLE_DIR/README.md"
test -f "$ZIP_PATH"

grep -q "xhs-local-helper-windows-tray-amd64.exe" "$BUNDLE_DIR/start-helper.bat"
grep -q "taskkill" "$BUNDLE_DIR/stop-helper.bat"
grep -q "xiaohongshu-mcp-windows-amd64.zip" "$BUNDLE_DIR/README.md"
grep -q "xhs-local-helper-windows-tray-amd64.exe" "$BUNDLE_DIR/README.md"
file "$BUNDLE_DIR/xhs-local-helper-windows-amd64.exe" | grep -Eq "PE32\\+|MS Windows"
file "$BUNDLE_DIR/xhs-local-helper-windows-tray-amd64.exe" | grep -Eq "PE32\\+|MS Windows"
