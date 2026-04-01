package applauncher

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/config"
)

const (
	appFolderName = "XhsLocalHelper"
)

type Config struct {
	HomeDir     string
	ArchivePath string
	HelperPath  string
}

type LaunchOutcome string

const (
	LaunchOutcomeStarted        LaunchOutcome = "started"
	LaunchOutcomeAlreadyRunning LaunchOutcome = "already_running"
)

func Prepare(cfg Config) (string, error) {
	if cfg.HomeDir == "" {
		return "", fmt.Errorf("home dir is required")
	}
	if cfg.ArchivePath == "" {
		return "", fmt.Errorf("archive path is required")
	}
	if cfg.HelperPath == "" {
		return "", fmt.Errorf("helper path is required")
	}
	if _, err := os.Stat(cfg.ArchivePath); err != nil {
		return "", fmt.Errorf("archive not found: %s", cfg.ArchivePath)
	}

	appDir := AppDir(cfg.HomeDir)
	binDir := filepath.Join(appDir, "bin")
	logDir := filepath.Join(appDir, "logs")
	runDir := filepath.Join(appDir, "run")
	tmpDir := filepath.Join(appDir, "tmp")

	for _, dir := range []string{appDir, binDir, logDir, runDir, tmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	names := macBinaryNames()
	if !fileExists(filepath.Join(binDir, names.MCP)) || !fileExists(filepath.Join(binDir, names.Login)) {
		if err := installArchive(cfg.ArchivePath, binDir); err != nil {
			return "", err
		}
	}

	return cfg.HelperPath, nil
}

func ExistingHelperRunning(baseURL string) bool {
	if baseURL == "" {
		return false
	}

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(baseURL + "/status")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

func StartHelper(helperPath, baseURL string) (LaunchOutcome, error) {
	if ExistingHelperRunning(baseURL) {
		return LaunchOutcomeAlreadyRunning, nil
	}

	cmd := exec.Command(helperPath)
	cmd.Env = os.Environ()
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("run helper: %w", err)
	}
	_ = cmd.Process.Release()
	return LaunchOutcomeStarted, nil
}

func AppDir(homeDir string) string {
	return filepath.Join(homeDir, "Library", "Application Support", appFolderName)
}

func ClearAllAccounts(homeDir string) error {
	appDir := AppDir(homeDir)
	for _, path := range []string{
		filepath.Join(appDir, "cookies.json"),
		filepath.Join(appDir, "cookies.txt"),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return ResetRunState(homeDir)
}

func ResetRunState(homeDir string) error {
	appDir := AppDir(homeDir)
	for _, path := range []string{
		filepath.Join(appDir, "run", "mcp.pid"),
		filepath.Join(appDir, "run", "login.pid"),
	} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	return nil
}

func StopManagedProcesses() {
	names := macBinaryNames()
	for _, pattern := range []string{names.Helper, names.MCP, names.Login} {
		cmd := exec.Command("pkill", "-f", pattern)
		_ = cmd.Run()
	}
}

func EnsureHelperStarted(cfg Config, baseURL string) (LaunchOutcome, error) {
	targetPath, err := Prepare(cfg)
	if err != nil {
		return "", err
	}
	return StartHelper(targetPath, baseURL)
}

func installArchive(archivePath, binDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var extracted int
	names := macBinaryNames()
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}

		name := filepath.Base(header.Name)
		switch name {
		case names.MCP, names.Login:
			target := filepath.Join(binDir, name)
			if err := writeExecutable(target, tr); err != nil {
				return err
			}
			extracted++
		}
	}

	if extracted < 2 {
		return fmt.Errorf("archive missing binaries, extracted=%d", extracted)
	}

	return nil
}

func macBinaryNames() config.MacBinaryNames {
	return config.CurrentMacBinaryNames()
}

func writeExecutable(path string, content io.Reader) error {
	tmpPath := path + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("open %s: %w", tmpPath, err)
	}
	if _, err := io.Copy(file, content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
