# AGENTS

Frontend project location: `~/IdeaProjects/aigc-platform-ui/`

Packaging notes:

- Build helper binary: `dist/xhs-local-helper-darwin-arm64`
- Build Intel helper binary: `dist/xhs-local-helper-darwin-amd64`
- Package `.app`: `scripts/package-app.sh`
- Verify `.app` structure: `scripts/verify-app-bundle.sh`
- Generated app bundle: `dist/XHS Local Helper.app`
- Generated distributable zip: `dist/xhs-local-helper-app.zip`
- Generated Intel app bundle: `dist/XHS Local Helper Intel.app`
- Generated Intel distributable zip: `dist/xhs-local-helper-intel-app.zip`
- Windows lane root: `windows/`
- Windows package script: `scripts/windows/package-bundle.sh`
- Windows verify script: `scripts/windows/verify-bundle.sh`
- Windows bundle output: `dist/windows-x64/xhs-local-helper-windows-x64/`
- Windows distributable zip: `dist/windows-x64/xhs-local-helper-windows-x64.zip`
- Windows tray icon asset in bundle: `tray-icon.ico`
- Windows MSI package script: `scripts/windows/package-msi.ps1`
- Windows MSI verify script: `scripts/windows/verify-msi.ps1`
- Windows MSI output: `dist/windows-x64/xhs-local-helper-windows-x64.msi`

Current delivery requirements:

- macOS Apple Silicon packaging: `TARGET_ARCH=arm64 ./scripts/package-app.sh`
- macOS Intel packaging: `TARGET_ARCH=amd64 ./scripts/package-app.sh`
- Apple Silicon archive source: `~/Downloads/xiaohongshu-mcp-darwin-arm64.tar.gz`
- Intel archive source: `~/Downloads/xiaohongshu-mcp-darwin-amd64.tar.gz`
- The generated `.app` contains both the helper binary and the upstream `xiaohongshu-mcp` archive
- First launch auto-installs upstream binaries into `~/Library/Application Support/XhsLocalHelper/bin`
- The `.app` is currently unsigned; on first launch macOS may require right-click `Open` or manual allow in Privacy & Security
- Windows x64 lane is isolated and currently packages against `xiaohongshu-mcp-windows-amd64.zip`
- Windows upstream archive lookup order: `WINDOWS_ARCHIVE_SRC` -> `windows/upstream/xiaohongshu-mcp-windows-amd64.zip` -> `~/Downloads/xiaohongshu-mcp-windows-amd64.zip`
- Windows MSI packaging currently requires a Windows host with the `wix` CLI in `PATH`
- Windows tray host is the primary user entrypoint and currently exposes 4 menu items: `chiccify小红书发布小助手`, `打开网页`, `清空所有账号`, `退出小助手`
- Windows customer and maintainer handoff docs live in `windows/README.md`
