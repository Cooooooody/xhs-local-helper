package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liuhaotian/xhs-local-helper/internal/windowsbundle"
)

//go:embed assets/tray-icon.ico
var trayIcon []byte

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: generate-windows-bundle-assets <bundle-dir>")
		os.Exit(1)
	}

	bundleDir := os.Args[1]
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create bundle dir: %v\n", err)
		os.Exit(1)
	}

	files := map[string]string{
		"start-helper.bat": windowsbundle.RenderStartHelperBat(),
		"stop-helper.bat":  windowsbundle.RenderStopHelperBat(),
		"README.md":        windowsbundle.RenderBundleReadme(),
	}
	for name, content := range files {
		path := filepath.Join(bundleDir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	iconPath := filepath.Join(bundleDir, windowsbundle.TrayIconFileName)
	if err := os.WriteFile(iconPath, trayIcon, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", windowsbundle.TrayIconFileName, err)
		os.Exit(1)
	}
}
