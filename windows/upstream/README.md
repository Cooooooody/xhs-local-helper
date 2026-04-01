# Upstream Windows MCP Input

Place the manually downloaded upstream Windows x64 archive here if you want a repo-local input:

- `xiaohongshu-mcp-windows-amd64.zip`

The Windows bundle script resolves the upstream archive in this order:

1. `WINDOWS_ARCHIVE_SRC` if provided
2. `windows/upstream/xiaohongshu-mcp-windows-amd64.zip`
3. `~/Downloads/xiaohongshu-mcp-windows-amd64.zip`

This keeps the Windows intake path isolated from the current macOS archive assumptions.
