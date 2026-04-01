#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
DIST_DIR="$ROOT_DIR/dist"
TARGET_ARCH="${TARGET_ARCH:-arm64}"
case "$TARGET_ARCH" in
  arm64)
    APP_NAME="XHS Local Helper.app"
    ZIP_PATH="$DIST_DIR/xhs-local-helper-app.zip"
    MENU_APP_BIN_SRC="$DIST_DIR/xhs-local-helper-menubar-app-arm64"
    GO_MAC_ARCH="arm64"
    SWIFT_TARGET_ARCH="arm64"
    HELPER_BIN_NAME="xhs-local-helper-darwin-arm64"
    SUPPORT_BIN_NAME="xhs-local-helper-app-support-darwin-arm64"
    ARCHIVE_NAME="xiaohongshu-mcp-darwin-arm64.tar.gz"
    ;;
  amd64)
    APP_NAME="XHS Local Helper Intel.app"
    ZIP_PATH="$DIST_DIR/xhs-local-helper-intel-app.zip"
    MENU_APP_BIN_SRC="$DIST_DIR/xhs-local-helper-menubar-app-amd64"
    GO_MAC_ARCH="amd64"
    SWIFT_TARGET_ARCH="x86_64"
    HELPER_BIN_NAME="xhs-local-helper-darwin-amd64"
    SUPPORT_BIN_NAME="xhs-local-helper-app-support-darwin-amd64"
    ARCHIVE_NAME="xiaohongshu-mcp-darwin-amd64.tar.gz"
    ;;
  *)
    echo "unsupported TARGET_ARCH: $TARGET_ARCH" >&2
    exit 1
    ;;
esac

APP_PATH="$DIST_DIR/$APP_NAME"
CONTENTS_DIR="$APP_PATH/Contents"
MACOS_DIR="$CONTENTS_DIR/MacOS"
RESOURCES_DIR="$CONTENTS_DIR/Resources"
HELPER_BIN_SRC="$DIST_DIR/$HELPER_BIN_NAME"
SUPPORT_BIN_SRC="$DIST_DIR/$SUPPORT_BIN_NAME"
ARCHIVE_SRC="${HOME}/Downloads/$ARCHIVE_NAME"
GO_BIN="${GO_BIN:-go}"
SWIFT_BIN="${SWIFT_BIN:-swiftc}"
MACOS_DEPLOYMENT_TARGET="${MACOS_DEPLOYMENT_TARGET:-12.0}"

mkdir -p "$DIST_DIR"

GOOS=darwin GOARCH="$GO_MAC_ARCH" "$GO_BIN" build -o "$HELPER_BIN_SRC" ./cmd/xhs-local-helper
GOOS=darwin GOARCH="$GO_MAC_ARCH" "$GO_BIN" build -o "$SUPPORT_BIN_SRC" ./cmd/xhs-local-helper-app-launcher
swift "$ROOT_DIR/scripts/generate-menubar-icon.swift"
MACOSX_DEPLOYMENT_TARGET="$MACOS_DEPLOYMENT_TARGET" \
  "$SWIFT_BIN" -O -target "${SWIFT_TARGET_ARCH}-apple-macos${MACOS_DEPLOYMENT_TARGET}" -framework Cocoa -o "$MENU_APP_BIN_SRC" "$ROOT_DIR/macos/MenuBarApp/main.swift"

if [ ! -x "$HELPER_BIN_SRC" ]; then
  echo "missing helper binary: $HELPER_BIN_SRC" >&2
  exit 1
fi

if [ ! -x "$SUPPORT_BIN_SRC" ]; then
  echo "missing support binary: $SUPPORT_BIN_SRC" >&2
  exit 1
fi

if [ ! -x "$MENU_APP_BIN_SRC" ]; then
  echo "missing menu app binary: $MENU_APP_BIN_SRC" >&2
  exit 1
fi

if [ ! -f "$ARCHIVE_SRC" ]; then
  echo "missing bundled archive: $ARCHIVE_SRC" >&2
  exit 1
fi

rm -rf "$APP_PATH"
mkdir -p "$MACOS_DIR" "$RESOURCES_DIR"

cp "$HELPER_BIN_SRC" "$RESOURCES_DIR/$HELPER_BIN_NAME"
cp "$SUPPORT_BIN_SRC" "$RESOURCES_DIR/$SUPPORT_BIN_NAME"
cp "$ROOT_DIR/macos/MenuBarApp/menubar-icon.png" "$RESOURCES_DIR/menubar-icon.png"
cp "$ARCHIVE_SRC" "$RESOURCES_DIR/$ARCHIVE_NAME"
cp "$MENU_APP_BIN_SRC" "$MACOS_DIR/XHS Local Helper"
chmod +x "$RESOURCES_DIR/$HELPER_BIN_NAME"
chmod +x "$RESOURCES_DIR/$SUPPORT_BIN_NAME"
chmod +x "$MACOS_DIR/XHS Local Helper"

cat > "$CONTENTS_DIR/Info.plist" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleDisplayName</key>
  <string>XHS Local Helper</string>
  <key>CFBundleExecutable</key>
  <string>XHS Local Helper</string>
  <key>CFBundleIdentifier</key>
  <string>com.liuhaotian.xhs-local-helper</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>XHS Local Helper</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>0.1.0</string>
  <key>CFBundleVersion</key>
  <string>1</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>LSUIElement</key>
  <true/>
</dict>
</plist>
PLIST

codesign --force --deep --sign - "$APP_PATH"

rm -f "$ZIP_PATH"
ditto -c -k --sequesterRsrc --keepParent "$APP_PATH" "$ZIP_PATH"

echo "Created app bundle: $APP_PATH"
echo "Created zip: $ZIP_PATH"
