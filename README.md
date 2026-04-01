# XHS Local Helper

`xhs-local-helper` 是一个本地轻量 Helper，用来在用户机器上托管 GitHub 版 `xiaohongshu-mcp` 二进制，并通过 localhost HTTP API 给网页前端调用。

当前骨架目标：

- 管理 `xiaohongshu-mcp-darwin-arm64`
- 管理 `xiaohongshu-login-darwin-arm64`
- 暴露最小接口：`/status` `/install` `/login` `/publish`
- 把登录态留在用户本机

## 目录约定

运行后默认使用：

- App 根目录：`~/Library/Application Support/XhsLocalHelper`
- 二进制目录：`~/Library/Application Support/XhsLocalHelper/bin`
- 日志目录：`~/Library/Application Support/XhsLocalHelper/logs`
- PID 目录：`~/Library/Application Support/XhsLocalHelper/run`

## API

### `GET /status`

返回本地 Helper 和 MCP 的基础状态。

响应示例：

```json
{
  "success": true,
  "code": 200,
  "installed": true,
  "mcpRunning": true,
  "loggedIn": true
}
```

### `POST /install`

从本地压缩包安装 GitHub 版二进制。

请求体：

```json
{
  "archivePath": "/Users/xxx/Downloads/xiaohongshu-mcp-darwin-arm64.tar.gz"
}
```

如果不传，默认读取：

```text
~/Downloads/xiaohongshu-mcp-darwin-arm64.tar.gz
```

### `POST /login`

启动 `xiaohongshu-login-darwin-arm64`。

### `POST /publish`

请求体：

```json
{
  "title": "今日妆照",
  "content": "今日妆照",
  "images": [
    "https://example.com/test.png"
  ]
}
```

## 返回码

当前 Helper 在 JSON 里补充了前端可直接判断的三位 `code`：

- `200` 成功
- `400` 请求体或参数无效
- `405` 方法不允许
- `412` `images` 为空
- `414` 图片来源无效
- `415` 图片下载失败
- `416` 图片预处理失败
- `430` 安装失败
- `440` 登录启动失败
- `450` MCP 运行时缺失
- `451` MCP 未就绪
- `452` MCP session 初始化失败
- `453` 上游 MCP 发布失败
- `500` 未分类内部错误

## 本地运行

已在当前仓库完成本地测试验证。可使用系统 Go，或仓库内本地工具链 `.tools/go/bin/go` 运行：

```bash
cd /Users/liuhaotian/IdeaProjects/xhs-local-helper
go run ./cmd/xhs-local-helper
```

默认监听：

```text
127.0.0.1:19180
```

## 给同事使用

当前已提供一个可直接分发的 macOS `.app`：

- `dist/XHS Local Helper.app`
- `dist/xhs-local-helper-app.zip`
- `dist/XHS Local Helper Intel.app`
- `dist/xhs-local-helper-intel-app.zip`

同事使用方式：

1. 解压 `dist/xhs-local-helper-app.zip`
2. 双击 `XHS Local Helper.app`
3. 首次启动时，应用会自动把 bundled 的上游二进制安装到：

```text
~/Library/Application Support/XhsLocalHelper/bin
```

4. 启动后本地 Helper 默认监听：

```text
127.0.0.1:19180
```

5. 可用以下命令检查状态：

```bash
curl -s http://127.0.0.1:19180/status
```

注意：

- `dist/XHS Local Helper.app` 目标是 macOS Apple Silicon (`arm64`)
- `dist/XHS Local Helper Intel.app` 目标是 macOS Intel (`amd64`)
- 当前 `.app` 还没有签名，首次打开时 macOS 可能会拦截；可右键 `Open`，或在系统设置的 Privacy & Security 中手动允许

## Windows X64

当前仓库已经单独拉出一条 Windows x64 lane，不影响现有 macOS `.app`：

- Windows 目录说明：`windows/`
- Windows 上游输入说明：`windows/upstream/README.md`
- Windows 交付与维护说明：`windows/README.md`
- Windows 打包脚本：`scripts/windows/package-bundle.sh`
- Windows 校验脚本：`scripts/windows/verify-bundle.sh`

默认情况下，Windows 打包脚本会按以下顺序查找上游 MCP 压缩包：

1. `WINDOWS_ARCHIVE_SRC`
2. `windows/upstream/xiaohongshu-mcp-windows-amd64.zip`
3. `~/Downloads/xiaohongshu-mcp-windows-amd64.zip`

运行：

```bash
./scripts/windows/package-bundle.sh
```

会生成：

- `dist/windows-x64/xhs-local-helper-windows-amd64.exe`
- `dist/windows-x64/xhs-local-helper-windows-tray-amd64.exe`
- `dist/windows-x64/xhs-local-helper-windows-x64/`
- `dist/windows-x64/xhs-local-helper-windows-x64.zip`

其中 bundle 内包含：

- `xhs-local-helper-windows-tray-amd64.exe`
- `xhs-local-helper-windows-amd64.exe`
- `xiaohongshu-mcp-windows-amd64.zip`
- `tray-icon.ico`
- `start-helper.bat`
- `stop-helper.bat`
- `README.md`

当前 Windows x64 bundle 以 tray host 为优先入口：

- `start-helper.bat` 会优先启动 `xhs-local-helper-windows-tray-amd64.exe`
- tray host 负责复用或拉起 helper、自动请求 `/install`，并提供托盘菜单动作
- tray host 没有主窗口，会常驻在系统托盘区
- 托盘菜单当前包含：
  - `chiccify小红书发布小助手`
  - `打开网页`
  - `清空所有账号`
  - `退出小助手`

Windows 本地运行目录：

```text
%LocalAppData%\XhsLocalHelper
```

Windows 账号 cookie 默认位于：

```text
%LocalAppData%\XhsLocalHelper\cookies.json
%LocalAppData%\XhsLocalHelper\cookies.txt
```

## 开发输入

- 开发调试时，仓库内已放置一份上游二进制样本：`third_party/xiaohongshu-mcp-darwin-arm64/`
- 运行时 `/install` 仍以本地压缩包为准：优先使用请求体里的 `archivePath`，否则按当前目标架构回退到下载目录中的对应压缩包

## 打包 `.app`

当前仓库已提供 `.app` 打包脚本：

```bash
./scripts/package-app.sh
```

按架构打包：

```bash
TARGET_ARCH=arm64 ./scripts/package-app.sh
TARGET_ARCH=amd64 ./scripts/package-app.sh
```

脚本依赖：

- 已编译的 `dist/xhs-local-helper-darwin-arm64`
- 本机下载目录中的 `~/Downloads/xiaohongshu-mcp-darwin-arm64.tar.gz`
- 已编译的 `dist/xhs-local-helper-darwin-amd64`
- 本机下载目录中的 `~/Downloads/xiaohongshu-mcp-darwin-amd64.tar.gz`

打包完成后会生成：

- `dist/XHS Local Helper.app`
- `dist/xhs-local-helper-app.zip`
- `dist/XHS Local Helper Intel.app`
- `dist/xhs-local-helper-intel-app.zip`

可用以下命令验证 `.app` 结构：

```bash
./scripts/verify-app-bundle.sh "dist/XHS Local Helper.app" arm64
./scripts/verify-app-bundle.sh "dist/XHS Local Helper Intel.app" amd64
```

## 已知限制

- Intel 版 `.app` 仍需要真实 Intel Mac 做最终启动与发布验收
- `/publish` 默认直传 `images` 给本地 `xiaohongshu-mcp`
- 目前只做了最小 CORS 白名单示例，后续需要接入真正的来源校验和短时 token
- 还没有做自动升级、签名安装、自启动和日志回传
