#!/bin/sh
set -eu

SOURCE_FILE="${1:-macos/MenuBarApp/main.swift}"

grep -q 'private let openWebMenuTitle = "chiccify小红书发布小助手"' "$SOURCE_FILE"
grep -q 'private let openWebShortcutMenuTitle = "打开网页"' "$SOURCE_FILE"
grep -q 'menu.addItem(makeMenuItem(title: openWebMenuTitle, action: #selector(openWebpage)))' "$SOURCE_FILE"
grep -q 'menu.addItem(makeMenuItem(title: openWebShortcutMenuTitle, action: #selector(openWebpage)))' "$SOURCE_FILE"
