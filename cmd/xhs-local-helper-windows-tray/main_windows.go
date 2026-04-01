//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/liuhaotian/xhs-local-helper/internal/windowstray"
)

func main() {
	archiveFlag := flag.String("archive", "", "path to bundled upstream archive")
	flag.Parse()

	execPath, err := os.Executable()
	if err != nil {
		fail(err)
	}
	bundleDir := filepath.Dir(execPath)
	helperPath := filepath.Join(bundleDir, "xhs-local-helper-windows-amd64.exe")
	archivePath := *archiveFlag
	if archivePath == "" {
		archivePath = filepath.Join(bundleDir, "xiaohongshu-mcp-windows-amd64.zip")
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		fail(fmt.Errorf("LOCALAPPDATA is not defined"))
	}
	homeDir := os.Getenv("USERPROFILE")
	if homeDir == "" {
		fail(fmt.Errorf("USERPROFILE is not defined"))
	}

	runtimeCfg := windowstray.DefaultRuntimeConfig(homeDir, localAppData, helperPath, archivePath, "http://127.0.0.1:19180")
	app := &windowstray.App{
		Controller: runtimeCfg.Controller(),
		AppTitle:   "XHS Local Helper",
		IconPath:   filepath.Join(bundleDir, "tray-icon.ico"),
	}
	if err := app.Run(); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
