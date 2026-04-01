package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultPort        = "19180"
	defaultMcpPort     = "18060"
	windowsArchive     = "xiaohongshu-mcp-windows-amd64.zip"
	windowsMcpBinary   = "xiaohongshu-mcp-windows-amd64.exe"
	windowsLoginBinary = "xiaohongshu-login-windows-amd64.exe"
)

type MacBinaryNames struct {
	Archive string
	MCP     string
	Login   string
	Helper  string
	Support string
}

type Config struct {
	ListenAddr      string
	AppDir          string
	BinDir          string
	RunDir          string
	LogDir          string
	TmpDir          string
	DefaultArchive  string
	McpBinaryName   string
	LoginBinaryName string
	McpBaseURL      string
	McpPort         string
}

func Load() (Config, error) {
	targetOS := effectiveOS()
	home, err := resolveHomeDir(targetOS)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		ListenAddr:      "127.0.0.1:" + valueOrDefault("XHS_HELPER_PORT", defaultPort),
		McpPort:         valueOrDefault("XHS_MCP_PORT", defaultMcpPort),
	}

	appDir := macAppDir(home)
	macNames := CurrentMacBinaryNames()
	cfg.DefaultArchive = filepath.Join(home, "Downloads", macNames.Archive)
	cfg.McpBinaryName = macNames.MCP
	cfg.LoginBinaryName = macNames.Login
	if targetOS == "windows" {
		appDir = windowsAppDir()
		cfg.DefaultArchive = filepath.Join(home, "Downloads", windowsArchive)
		cfg.McpBinaryName = windowsMcpBinary
		cfg.LoginBinaryName = windowsLoginBinary
	}

	cfg.AppDir = appDir
	cfg.BinDir = filepath.Join(appDir, "bin")
	cfg.RunDir = filepath.Join(appDir, "run")
	cfg.LogDir = filepath.Join(appDir, "logs")
	cfg.TmpDir = filepath.Join(appDir, "tmp")
	cfg.McpBaseURL = "http://127.0.0.1:" + cfg.McpPort + "/mcp"
	return cfg, nil
}

func effectiveOS() string {
	override := strings.TrimSpace(strings.ToLower(os.Getenv("XHS_HELPER_TARGET_OS")))
	if override != "" {
		return override
	}
	return runtime.GOOS
}

func effectiveArch() string {
	override := strings.TrimSpace(strings.ToLower(os.Getenv("XHS_HELPER_TARGET_ARCH")))
	if override != "" {
		switch override {
		case "x86_64":
			return "amd64"
		case "aarch64":
			return "arm64"
		default:
			return override
		}
	}
	switch runtime.GOARCH {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

func CurrentMacBinaryNames() MacBinaryNames {
	if effectiveArch() == "amd64" {
		return MacBinaryNames{
			Archive: "xiaohongshu-mcp-darwin-amd64.tar.gz",
			MCP:     "xiaohongshu-mcp-darwin-amd64",
			Login:   "xiaohongshu-login-darwin-amd64",
			Helper:  "xhs-local-helper-darwin-amd64",
			Support: "xhs-local-helper-app-support-darwin-amd64",
		}
	}
	return MacBinaryNames{
		Archive: "xiaohongshu-mcp-darwin-arm64.tar.gz",
		MCP:     "xiaohongshu-mcp-darwin-arm64",
		Login:   "xiaohongshu-login-darwin-arm64",
		Helper:  "xhs-local-helper-darwin-arm64",
		Support: "xhs-local-helper-app-support-darwin-arm64",
	}
}

func resolveHomeDir(targetOS string) (string, error) {
	if targetOS == "windows" {
		if home := strings.TrimSpace(os.Getenv("USERPROFILE")); home != "" {
			return home, nil
		}
		return "", fmt.Errorf("resolve user home: USERPROFILE is not defined")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	return home, nil
}

func macAppDir(home string) string {
	return filepath.Join(home, "Library", "Application Support", "XhsLocalHelper")
}

func windowsAppDir() string {
	localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA"))
	if localAppData == "" {
		localAppData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	return filepath.Join(localAppData, "XhsLocalHelper")
}

func valueOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
