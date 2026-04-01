package config

import (
	"path/filepath"
	"testing"
)

func TestLoadUsesWindowsDefaultsWhenTargetOSIsWindows(t *testing.T) {
	t.Setenv("XHS_HELPER_TARGET_OS", "windows")
	t.Setenv("LOCALAPPDATA", `C:\Users\tester\AppData\Local`)
	t.Setenv("USERPROFILE", `C:\Users\tester`)
	t.Setenv("HOME", "")
	t.Setenv("XHS_HELPER_PORT", "")
	t.Setenv("XHS_MCP_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	wantAppDir := filepath.Join(`C:\Users\tester\AppData\Local`, "XhsLocalHelper")
	if cfg.AppDir != wantAppDir {
		t.Fatalf("cfg.AppDir = %q, want %q", cfg.AppDir, wantAppDir)
	}
	if cfg.BinDir != filepath.Join(wantAppDir, "bin") {
		t.Fatalf("cfg.BinDir = %q", cfg.BinDir)
	}
	if cfg.DefaultArchive != filepath.Join(`C:\Users\tester`, "Downloads", "xiaohongshu-mcp-windows-amd64.zip") {
		t.Fatalf("cfg.DefaultArchive = %q", cfg.DefaultArchive)
	}
	if cfg.McpBinaryName != "xiaohongshu-mcp-windows-amd64.exe" {
		t.Fatalf("cfg.McpBinaryName = %q", cfg.McpBinaryName)
	}
	if cfg.LoginBinaryName != "xiaohongshu-login-windows-amd64.exe" {
		t.Fatalf("cfg.LoginBinaryName = %q", cfg.LoginBinaryName)
	}
}

func TestLoadUsesMacIntelDefaultsWhenTargetArchIsAmd64(t *testing.T) {
	t.Setenv("XHS_HELPER_TARGET_OS", "darwin")
	t.Setenv("XHS_HELPER_TARGET_ARCH", "amd64")
	t.Setenv("HOME", "/Users/tester")
	t.Setenv("XHS_HELPER_PORT", "")
	t.Setenv("XHS_MCP_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultArchive != filepath.Join("/Users/tester", "Downloads", "xiaohongshu-mcp-darwin-amd64.tar.gz") {
		t.Fatalf("cfg.DefaultArchive = %q", cfg.DefaultArchive)
	}
	if cfg.McpBinaryName != "xiaohongshu-mcp-darwin-amd64" {
		t.Fatalf("cfg.McpBinaryName = %q", cfg.McpBinaryName)
	}
	if cfg.LoginBinaryName != "xiaohongshu-login-darwin-amd64" {
		t.Fatalf("cfg.LoginBinaryName = %q", cfg.LoginBinaryName)
	}
}
