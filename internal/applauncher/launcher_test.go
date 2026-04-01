package applauncher

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestPrepareInstallsBundledBinariesAndCreatesRuntimeDirs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	archivePath := filepath.Join(root, "xiaohongshu-mcp-darwin-arm64.tar.gz")
	if err := writeArchive(archivePath, map[string]string{
		"xiaohongshu-mcp-darwin-arm64":   "#!/bin/sh\nexit 0\n",
		"xiaohongshu-login-darwin-arm64": "#!/bin/sh\nexit 0\n",
	}); err != nil {
		t.Fatalf("writeArchive() error = %v", err)
	}

	execPath, err := Prepare(Config{
		HomeDir:     root,
		ArchivePath: archivePath,
		HelperPath:  filepath.Join(root, "Resources", "xhs-local-helper-darwin-arm64"),
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if execPath != filepath.Join(root, "Resources", "xhs-local-helper-darwin-arm64") {
		t.Fatalf("execPath = %q", execPath)
	}

	for _, dir := range []string{
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper"),
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "bin"),
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "logs"),
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "run"),
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "tmp"),
	} {
		if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
			t.Fatalf("runtime dir missing: %s statErr=%v", dir, statErr)
		}
	}

	for _, binary := range []string{
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "bin", "xiaohongshu-mcp-darwin-arm64"),
		filepath.Join(root, "Library", "Application Support", "XhsLocalHelper", "bin", "xiaohongshu-login-darwin-arm64"),
	} {
		info, statErr := os.Stat(binary)
		if statErr != nil {
			t.Fatalf("binary missing: %s statErr=%v", binary, statErr)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("binary not executable: %s mode=%v", binary, info.Mode())
		}
	}
}

func TestPrepareRejectsMissingArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := Prepare(Config{
		HomeDir:     root,
		ArchivePath: filepath.Join(root, "missing.tar.gz"),
		HelperPath:  filepath.Join(root, "Resources", "xhs-local-helper-darwin-arm64"),
	})
	if err == nil {
		t.Fatal("Prepare() error = nil, want missing archive error")
	}
}

func TestExistingHelperRunningDetectsStatusEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if !ExistingHelperRunning(server.URL) {
		t.Fatal("ExistingHelperRunning() = false, want true")
	}
}

func TestExistingHelperRunningRejectsNonHelperEndpoint(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	if ExistingHelperRunning(server.URL) {
		t.Fatal("ExistingHelperRunning() = true, want false")
	}
}

func TestStartHelperReturnsAlreadyRunningWithoutLaunchingProcess(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/status" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	root := t.TempDir()
	helperPath := filepath.Join(root, "helper.sh")
	markerPath := filepath.Join(root, "started")
	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\n"+"touch "+shellQuote(markerPath)+"\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(helper.sh) error = %v", err)
	}

	outcome, err := StartHelper(helperPath, server.URL)
	if err != nil {
		t.Fatalf("StartHelper() error = %v", err)
	}
	if outcome != LaunchOutcomeAlreadyRunning {
		t.Fatalf("outcome = %q, want %q", outcome, LaunchOutcomeAlreadyRunning)
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("marker stat err = %v, want not exist", err)
	}
}

func TestStartHelperStartsProcessInBackground(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	helperPath := filepath.Join(root, "helper.sh")
	markerPath := filepath.Join(root, "started")
	if err := os.WriteFile(helperPath, []byte("#!/bin/sh\n"+"touch "+shellQuote(markerPath)+"\n"+"sleep 1\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(helper.sh) error = %v", err)
	}

	outcome, err := StartHelper(helperPath, "http://127.0.0.1:1")
	if err != nil {
		t.Fatalf("StartHelper() error = %v", err)
	}
	if outcome != LaunchOutcomeStarted {
		t.Fatalf("outcome = %q, want %q", outcome, LaunchOutcomeStarted)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(markerPath); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("helper marker %q was not created", markerPath)
}

func TestClearAllAccountsRemovesCookiesAndRunState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	appDir := AppDir(root)
	runDir := filepath.Join(appDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(runDir) error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(appDir, "cookies.json"),
		filepath.Join(appDir, "cookies.txt"),
		filepath.Join(runDir, "mcp.pid"),
		filepath.Join(runDir, "login.pid"),
	} {
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	if err := ClearAllAccounts(root); err != nil {
		t.Fatalf("ClearAllAccounts() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(appDir, "cookies.json"),
		filepath.Join(appDir, "cookies.txt"),
		filepath.Join(runDir, "mcp.pid"),
		filepath.Join(runDir, "login.pid"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stat(%s) err = %v, want not exist", path, err)
		}
	}
}

func TestResetRunStateKeepsCookiesButRemovesPidFiles(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	appDir := AppDir(root)
	runDir := filepath.Join(appDir, "run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(runDir) error = %v", err)
	}
	cookiesPath := filepath.Join(appDir, "cookies.json")
	if err := os.WriteFile(cookiesPath, []byte("data"), 0o644); err != nil {
		t.Fatalf("WriteFile(cookies.json) error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(runDir, "mcp.pid"),
		filepath.Join(runDir, "login.pid"),
	} {
		if err := os.WriteFile(path, []byte("123"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", path, err)
		}
	}

	if err := ResetRunState(root); err != nil {
		t.Fatalf("ResetRunState() error = %v", err)
	}

	if _, err := os.Stat(cookiesPath); err != nil {
		t.Fatalf("cookies stat err = %v, want file to remain", err)
	}
	for _, path := range []string{
		filepath.Join(runDir, "mcp.pid"),
		filepath.Join(runDir, "login.pid"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("stat(%s) err = %v, want not exist", path, err)
		}
	}
}

func TestMacBinaryNamesFollowTargetArch(t *testing.T) {
	t.Setenv("XHS_HELPER_TARGET_ARCH", "amd64")

	names := macBinaryNames()
	if names.Helper != "xhs-local-helper-darwin-amd64" {
		t.Fatalf("helper = %q", names.Helper)
	}
	if names.MCP != "xiaohongshu-mcp-darwin-amd64" {
		t.Fatalf("mcp = %q", names.MCP)
	}
	if names.Login != "xiaohongshu-login-darwin-amd64" {
		t.Fatalf("login = %q", names.Login)
	}
	if names.Archive != "xiaohongshu-mcp-darwin-amd64.tar.gz" {
		t.Fatalf("archive = %q", names.Archive)
	}
}

func TestMacBinaryNamesDefaultToCurrentArch(t *testing.T) {
	t.Setenv("XHS_HELPER_TARGET_ARCH", "")

	names := macBinaryNames()
	if runtime.GOARCH == "amd64" {
		if names.Helper != "xhs-local-helper-darwin-amd64" {
			t.Fatalf("helper = %q", names.Helper)
		}
		return
	}
	if names.Helper != "xhs-local-helper-darwin-arm64" {
		t.Fatalf("helper = %q", names.Helper)
	}
}

func writeArchive(path string, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	gzw := gzip.NewWriter(file)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	for name, body := range files {
		data := []byte(body)
		header := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}

	return nil
}

func shellQuote(value string) string {
	return "'" + value + "'"
}
