package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/applauncher"
	"github.com/liuhaotian/xhs-local-helper/internal/config"
)

func main() {
	if len(os.Args) != 2 {
		fail(fmt.Errorf("usage: %s <ensure-started|clear-accounts|stop-all>", filepath.Base(os.Args[0])))
	}

	cfg, baseURL, err := launcherConfig()
	if err != nil {
		fail(err)
	}

	switch os.Args[1] {
	case "ensure-started":
		outcome, err := applauncher.EnsureHelperStarted(cfg, baseURL)
		if err != nil {
			fail(err)
		}
		fmt.Println(outcome)
	case "clear-accounts":
		if err := applauncher.ClearAllAccounts(cfg.HomeDir); err != nil {
			fail(err)
		}
		applauncher.StopManagedProcesses()
		time.Sleep(300 * time.Millisecond)
		if _, err := applauncher.EnsureHelperStarted(cfg, baseURL); err != nil {
			fail(err)
		}
	case "stop-all":
		applauncher.StopManagedProcesses()
		if err := applauncher.ResetRunState(cfg.HomeDir); err != nil {
			fail(err)
		}
	default:
		fail(fmt.Errorf("unknown command: %s", os.Args[1]))
	}
}

func launcherConfig() (applauncher.Config, string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return applauncher.Config{}, "", fmt.Errorf("resolve executable: %w", err)
	}

	resourcesDir := filepath.Dir(execPath)
	names := config.CurrentMacBinaryNames()
	helperPath := filepath.Join(resourcesDir, names.Helper)
	archivePath := filepath.Join(resourcesDir, names.Archive)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return applauncher.Config{}, "", fmt.Errorf("resolve home dir: %w", err)
	}

	port := os.Getenv("XHS_HELPER_PORT")
	if port == "" {
		port = "19180"
	}

	return applauncher.Config{
		HomeDir:     homeDir,
		ArchivePath: archivePath,
		HelperPath:  helperPath,
	}, "http://127.0.0.1:" + port, nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
