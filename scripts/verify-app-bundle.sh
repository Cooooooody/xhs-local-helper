#!/bin/sh
set -eu

APP_PATH="${1:-dist/XHS Local Helper.app}"
TARGET_ARCH="${2:-arm64}"
EXPECT_NOTARIZED="${EXPECT_NOTARIZED:-0}"
EXPECTED_IDENTITY="${EXPECTED_IDENTITY:-}"
MAIN_EXECUTABLE="$APP_PATH/Contents/MacOS/XHS Local Helper"
case "$TARGET_ARCH" in
  arm64)
    HELPER_BIN_NAME="xhs-local-helper-darwin-arm64"
    SUPPORT_BIN_NAME="xhs-local-helper-app-support-darwin-arm64"
    ARCHIVE_NAME="xiaohongshu-mcp-darwin-arm64.tar.gz"
    MACH_O_PATTERN="Mach-O 64-bit executable arm64"
    ;;
  amd64)
    HELPER_BIN_NAME="xhs-local-helper-darwin-amd64"
    SUPPORT_BIN_NAME="xhs-local-helper-app-support-darwin-amd64"
    ARCHIVE_NAME="xiaohongshu-mcp-darwin-amd64.tar.gz"
    MACH_O_PATTERN="Mach-O 64-bit executable x86_64"
    ;;
  *)
    echo "unsupported TARGET_ARCH: $TARGET_ARCH" >&2
    exit 1
    ;;
esac

test -d "$APP_PATH"
test -f "$APP_PATH/Contents/Info.plist"
test -x "$MAIN_EXECUTABLE"
test -f "$APP_PATH/Contents/Resources/menubar-icon.png"
test -f "$APP_PATH/Contents/Resources/$HELPER_BIN_NAME"
test -f "$APP_PATH/Contents/Resources/$SUPPORT_BIN_NAME"
test -f "$APP_PATH/Contents/Resources/$ARCHIVE_NAME"

grep -q "CFBundleExecutable" "$APP_PATH/Contents/Info.plist"
grep -q "XHS Local Helper" "$APP_PATH/Contents/Info.plist"
grep -q "<string>12.0</string>" "$APP_PATH/Contents/Info.plist"
file "$MAIN_EXECUTABLE" | grep -q "$MACH_O_PATTERN"
otool -l "$MAIN_EXECUTABLE" | grep -A3 "LC_BUILD_VERSION" | grep -q "minos 12.0"
codesign --verify --deep --strict --verbose=2 "$APP_PATH" >/dev/null 2>&1
if [ -n "$EXPECTED_IDENTITY" ]; then
  codesign -dvv "$APP_PATH" 2>&1 | grep -q "Authority=$EXPECTED_IDENTITY"
fi
if [ "$EXPECT_NOTARIZED" = "1" ]; then
  xcrun stapler validate "$APP_PATH" >/dev/null 2>&1
  spctl -a -vv "$APP_PATH" 2>&1 | grep -q "accepted"
fi
