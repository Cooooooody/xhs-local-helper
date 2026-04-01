#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)
DIST_DIR="$ROOT_DIR/dist/windows-x64"
BUNDLE_DIR="$DIST_DIR/xhs-local-helper-windows-x64"
HELPER_BIN="$DIST_DIR/xhs-local-helper-windows-amd64.exe"
TRAY_BIN="$DIST_DIR/xhs-local-helper-windows-tray-amd64.exe"
ZIP_PATH="$DIST_DIR/xhs-local-helper-windows-x64.zip"
REPO_ARCHIVE="$ROOT_DIR/windows/upstream/xiaohongshu-mcp-windows-amd64.zip"
DOWNLOAD_ARCHIVE="${HOME}/Downloads/xiaohongshu-mcp-windows-amd64.zip"
GO_BIN="${GO_BIN:-go}"

ARCHIVE_SRC="${WINDOWS_ARCHIVE_SRC:-}"
if [ -z "$ARCHIVE_SRC" ]; then
  if [ -f "$REPO_ARCHIVE" ]; then
    ARCHIVE_SRC="$REPO_ARCHIVE"
  else
    ARCHIVE_SRC="$DOWNLOAD_ARCHIVE"
  fi
fi

if [ ! -f "$ARCHIVE_SRC" ]; then
  echo "missing Windows upstream archive: $ARCHIVE_SRC" >&2
  echo "expected file: xiaohongshu-mcp-windows-amd64.zip" >&2
  exit 1
fi

mkdir -p "$DIST_DIR"
GOOS=windows GOARCH=amd64 "$GO_BIN" build -o "$HELPER_BIN" ./cmd/xhs-local-helper
GOOS=windows GOARCH=amd64 "$GO_BIN" build -o "$TRAY_BIN" ./cmd/xhs-local-helper-windows-tray

rm -rf "$BUNDLE_DIR"
mkdir -p "$BUNDLE_DIR"

cp "$HELPER_BIN" "$BUNDLE_DIR/xhs-local-helper-windows-amd64.exe"
cp "$TRAY_BIN" "$BUNDLE_DIR/xhs-local-helper-windows-tray-amd64.exe"
cp "$ARCHIVE_SRC" "$BUNDLE_DIR/xiaohongshu-mcp-windows-amd64.zip"
"$GO_BIN" run ./cmd/generate-windows-bundle-assets "$BUNDLE_DIR"

rm -f "$ZIP_PATH"
(
  cd "$DIST_DIR"
  zip -rq "$(basename "$ZIP_PATH")" "$(basename "$BUNDLE_DIR")"
)

echo "Created Windows bundle: $BUNDLE_DIR"
echo "Created Windows zip: $ZIP_PATH"
