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
    MCP_BIN_NAME="xiaohongshu-mcp-darwin-arm64"
    LOGIN_BIN_NAME="xiaohongshu-login-darwin-arm64"
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
    MCP_BIN_NAME="xiaohongshu-mcp-darwin-amd64"
    LOGIN_BIN_NAME="xiaohongshu-login-darwin-amd64"
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
DEVELOPER_IDENTITY="${DEVELOPER_IDENTITY:-}"
NOTARY_PROFILE="${NOTARY_PROFILE:-}"
ENABLE_NOTARIZATION="${ENABLE_NOTARIZATION:-0}"
TEMP_NOTARY_ZIP=""
TEMP_SIGNED_ARCHIVE=""
TEMP_ARCHIVE_DIR=""
TEMP_NOTARY_RESULT=""

cleanup() {
  if [ -n "${TEMP_NOTARY_ZIP:-}" ] && [ -f "$TEMP_NOTARY_ZIP" ]; then
    rm -f "$TEMP_NOTARY_ZIP"
  fi
  if [ -n "${TEMP_SIGNED_ARCHIVE:-}" ] && [ -f "$TEMP_SIGNED_ARCHIVE" ]; then
    rm -f "$TEMP_SIGNED_ARCHIVE"
  fi
  if [ -n "${TEMP_ARCHIVE_DIR:-}" ] && [ -d "$TEMP_ARCHIVE_DIR" ]; then
    rm -rf "$TEMP_ARCHIVE_DIR"
  fi
  if [ -n "${TEMP_NOTARY_RESULT:-}" ] && [ -f "$TEMP_NOTARY_RESULT" ]; then
    rm -f "$TEMP_NOTARY_RESULT"
  fi
}
trap cleanup EXIT

sign_macos_target() {
  target_path="$1"
  if [ -n "$DEVELOPER_IDENTITY" ]; then
    codesign \
      --force \
      --timestamp \
      --options runtime \
      --sign "$DEVELOPER_IDENTITY" \
      "$target_path"
    return
  fi
  codesign --force --sign - "$target_path"
}

prepare_bundled_archive() {
  source_archive="$1"
  if [ -z "$DEVELOPER_IDENTITY" ]; then
    echo "$source_archive"
    return
  fi

  TEMP_ARCHIVE_DIR="$(mktemp -d "$DIST_DIR/archive-sign.XXXXXX")"
  tar -xzf "$source_archive" -C "$TEMP_ARCHIVE_DIR"

  for bundled_binary in "$TEMP_ARCHIVE_DIR/$MCP_BIN_NAME" "$TEMP_ARCHIVE_DIR/$LOGIN_BIN_NAME"; do
    if [ -f "$bundled_binary" ]; then
      chmod +x "$bundled_binary"
      sign_macos_target "$bundled_binary"
    fi
  done

  TEMP_SIGNED_ARCHIVE="$(mktemp "$DIST_DIR/$ARCHIVE_NAME.XXXXXX")"
  (
    cd "$TEMP_ARCHIVE_DIR"
    tar -czf "$TEMP_SIGNED_ARCHIVE" ./*
  )
  echo "$TEMP_SIGNED_ARCHIVE"
}

submit_for_notarization() {
  submission_path="$1"
  TEMP_NOTARY_RESULT="$(mktemp "$DIST_DIR/notary-result.XXXXXX.json")"
  xcrun notarytool submit "$submission_path" --keychain-profile "$NOTARY_PROFILE" --wait --output-format json > "$TEMP_NOTARY_RESULT"

  submission_id="$(python3 - <<'PY' "$TEMP_NOTARY_RESULT"
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as fh:
    payload = json.load(fh)
print(payload.get("id", ""))
PY
)"
  submission_status="$(python3 - <<'PY' "$TEMP_NOTARY_RESULT"
import json, sys
with open(sys.argv[1], 'r', encoding='utf-8') as fh:
    payload = json.load(fh)
print(payload.get("status", ""))
PY
)"

  if [ "$submission_status" != "Accepted" ]; then
    echo "notarization status: $submission_status" >&2
    if [ -n "$submission_id" ]; then
      xcrun notarytool log "$submission_id" --keychain-profile "$NOTARY_PROFILE" >&2 || true
    fi
    exit 1
  fi
}

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

ARCHIVE_SRC="$(prepare_bundled_archive "$ARCHIVE_SRC")"

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

sign_macos_target "$RESOURCES_DIR/$HELPER_BIN_NAME"
sign_macos_target "$RESOURCES_DIR/$SUPPORT_BIN_NAME"
sign_macos_target "$MACOS_DIR/XHS Local Helper"
if [ -n "$DEVELOPER_IDENTITY" ]; then
  codesign \
    --force \
    --timestamp \
    --options runtime \
    --sign "$DEVELOPER_IDENTITY" \
    "$APP_PATH"
else
  codesign --force --deep --sign - "$APP_PATH"
fi

if [ "$ENABLE_NOTARIZATION" = "1" ]; then
  if [ -z "$DEVELOPER_IDENTITY" ]; then
    echo "ENABLE_NOTARIZATION=1 requires DEVELOPER_IDENTITY" >&2
    exit 1
  fi
  if [ -z "$NOTARY_PROFILE" ]; then
    echo "ENABLE_NOTARIZATION=1 requires NOTARY_PROFILE" >&2
    exit 1
  fi
  TEMP_NOTARY_ZIP="$(mktemp "$DIST_DIR/notary-submit.XXXXXX.zip")"
  ditto -c -k --sequesterRsrc --keepParent "$APP_PATH" "$TEMP_NOTARY_ZIP"
  submit_for_notarization "$TEMP_NOTARY_ZIP"
  xcrun stapler staple "$APP_PATH"
fi

rm -f "$ZIP_PATH"
ditto -c -k --sequesterRsrc --keepParent "$APP_PATH" "$ZIP_PATH"

echo "Created app bundle: $APP_PATH"
echo "Created zip: $ZIP_PATH"
