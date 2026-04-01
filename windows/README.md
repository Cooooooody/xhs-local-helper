# Windows X64 说明

这条 Windows x64 lane 用于交付和维护 Windows 版 `xhs-local-helper`。

当前交付形态有两种：

- zip bundle：`dist/windows-x64/xhs-local-helper-windows-x64.zip`
- MSI 安装器：`dist/windows-x64/xhs-local-helper-windows-x64.msi`

## 给客户/同事使用

### 方式一：使用 zip bundle

1. 解压 `xhs-local-helper-windows-x64.zip`
2. 双击 `start-helper.bat`
3. 程序会在 Windows 系统托盘区常驻，不会弹主窗口
4. 首次启动会自动确保本地 helper 和 bundled 的 `xiaohongshu-mcp-windows-amd64.zip` 可用
5. 然后网页前端即可访问：

```text
http://127.0.0.1:19180
```

bundle 内包含：

- `xhs-local-helper-windows-tray-amd64.exe`
- `xhs-local-helper-windows-amd64.exe`
- `xiaohongshu-mcp-windows-amd64.zip`
- `tray-icon.ico`
- `start-helper.bat`
- `stop-helper.bat`
- `README.md`

### 方式二：使用 MSI

1. 双击 `xhs-local-helper-windows-x64.msi`
2. 按安装向导完成安装
3. 从开始菜单启动 `XHS Local Helper`
4. 程序启动后会常驻在系统托盘区

说明：

- 当前 MSI 默认不删除 `%LocalAppData%\\XhsLocalHelper` 运行态数据
- 当前 MSI 仍未签名，正式客户分发前建议补代码签名

## Windows 托盘行为

Windows 版是 tray-first，不会弹主窗口。

托盘菜单当前包含 4 项：

- `chiccify小红书发布小助手`
- `打开网页`
- `清空所有账号`
- `退出小助手`

其中：

- `chiccify小红书发布小助手` 和 `打开网页` 都会打开：

```text
https://musegate.tech/#/text2img/auto-generation
```

- `清空所有账号` 会删除本机 cookie，并重置运行态
- `退出小助手` 会关闭 tray、helper、mcp、login 相关进程

## Windows 本地运行目录

当前 Windows 版运行目录位于：

```text
%LocalAppData%\XhsLocalHelper
```

常见路径包括：

- 根目录：`%LocalAppData%\\XhsLocalHelper`
- cookie：
  - `%LocalAppData%\\XhsLocalHelper\\cookies.json`
  - `%LocalAppData%\\XhsLocalHelper\\cookies.txt`
- 日志：`%LocalAppData%\\XhsLocalHelper\\logs`
- 运行态：`%LocalAppData%\\XhsLocalHelper\\run`
- 临时目录：`%LocalAppData%\\XhsLocalHelper\\tmp`

手动清除账号可执行：

```powershell
Remove-Item "$env:LOCALAPPDATA\XhsLocalHelper\cookies.json" -ErrorAction SilentlyContinue
Remove-Item "$env:LOCALAPPDATA\XhsLocalHelper\cookies.txt" -ErrorAction SilentlyContinue
```

## 给 Windows 维护者 / Codex 的打包说明

### 环境准备

至少需要：

- Go
- PowerShell 7
- dotnet
- WiX CLI（仅 MSI）

如果要在 Windows 机器上装 Codex CLI：

1. 安装 Node.js LTS
2. 如果 PowerShell 拦截 `npm.ps1`，优先使用 `npm.cmd`
3. 安装 Codex：

```powershell
npm.cmd install -g @openai/codex
codex.cmd --version
```

### 上游包准备

Windows 打包依赖上游归档：

- 文件名：`xiaohongshu-mcp-windows-amd64.zip`

查找顺序：

1. `WINDOWS_ARCHIVE_SRC`
2. `windows/upstream/xiaohongshu-mcp-windows-amd64.zip`
3. `~/Downloads/xiaohongshu-mcp-windows-amd64.zip`

### 先打 zip bundle

在仓库根目录执行：

```bash
./scripts/windows/package-bundle.sh
./scripts/windows/verify-bundle.sh
```

产物：

- `dist/windows-x64/xhs-local-helper-windows-amd64.exe`
- `dist/windows-x64/xhs-local-helper-windows-tray-amd64.exe`
- `dist/windows-x64/xhs-local-helper-windows-x64/`
- `dist/windows-x64/xhs-local-helper-windows-x64.zip`

### 再打 MSI

先确保 Windows 主机上 `wix` 可用：

```powershell
dotnet tool install --global wix
wix extension add -g WixToolset.Util.wixext/6.0.2
```

如果要指定 MSI 版本：

```powershell
$env:WINDOWS_MSI_VERSION = "0.1.0"
```

打包和验证：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\windows\package-msi.ps1
powershell -ExecutionPolicy Bypass -File .\scripts\windows\verify-msi.ps1
```

MSI 产物：

- `dist/windows-x64/xhs-local-helper-windows-x64.msi`

## 当前建议的真实验证

在真实 Windows x64 机器上至少确认：

- 双击 `start-helper.bat` 后托盘图标出现并常驻
- 托盘 4 个菜单项都能正常响应
- `http://127.0.0.1:19180/status` 返回正常 JSON
- 登录、发帖链路能拉起 bundled 的 `xiaohongshu-mcp-windows-amd64.zip`
- `stop-helper.bat` 能清理 helper 相关进程
- MSI 安装、启动、卸载流程正常

## 当前限制

- Windows 包当前仍未签名，正式发客户前建议补代码签名
- MSI 生成当前要求 Windows 主机，不建议在 macOS 上硬出 MSI
