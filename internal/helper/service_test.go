package helper

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/config"
	"github.com/liuhaotian/xhs-local-helper/internal/model"
)

func TestWaitForMcpReadyReturnsErrorWhenEndpointNeverResponds(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	svc, err := New(config.Config{
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  filepath.Join(root, "missing.tar.gz"),
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      "http://127.0.0.1:1/mcp",
		McpPort:         "1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	svc.httpClient = &http.Client{Timeout: 20 * time.Millisecond}

	err = svc.waitForMcpReady(100 * time.Millisecond)
	if err == nil {
		t.Fatal("waitForMcpReady() error = nil, want timeout error")
	}
}

func TestNewUsesThreeMinuteMcpRequestTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	svc, err := New(config.Config{
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  filepath.Join(root, "missing.tar.gz"),
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      "http://127.0.0.1:18060/mcp",
		McpPort:         "18060",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if got := svc.httpClient.Timeout; got != 3*time.Minute {
		t.Fatalf("httpClient.Timeout = %s, want %s", got, 3*time.Minute)
	}
}

func TestWaitForMcpReadySucceedsWhenEndpointResponds(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	root := t.TempDir()
	svc, err := New(config.Config{
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  filepath.Join(root, "missing.tar.gz"),
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      server.URL,
		McpPort:         "18060",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	svc.httpClient = server.Client()

	if err := svc.waitForMcpReady(100 * time.Millisecond); err != nil {
		t.Fatalf("waitForMcpReady() error = %v", err)
	}
}

func TestNewCreatesRuntimeDirectoriesAndIsIdempotent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(root)

	first, err := New(cfg)
	if err != nil {
		t.Fatalf("first New() error = %v", err)
	}

	for _, dir := range []string{cfg.AppDir, cfg.BinDir, cfg.RunDir, cfg.LogDir, cfg.TmpDir} {
		info, statErr := os.Stat(dir)
		if statErr != nil {
			t.Fatalf("Stat(%q) error = %v", dir, statErr)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", dir)
		}
	}

	cookies := filepath.Join(cfg.AppDir, "cookies.json")
	if err := os.WriteFile(cookies, []byte(`{"token":"x"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", cookies, err)
	}

	second, err := New(cfg)
	if err != nil {
		t.Fatalf("second New() error = %v", err)
	}

	if first.cookiesPath() != cookies || second.cookiesPath() != cookies {
		t.Fatalf("cookiesPath() = %q / %q, want %q", first.cookiesPath(), second.cookiesPath(), cookies)
	}
	if _, statErr := os.Stat(cookies); statErr != nil {
		t.Fatalf("cookie file disappeared after second New(): %v", statErr)
	}
}

func TestInstallUsesDefaultArchiveAndStatusReflectsInstalled(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	defaultArchive := filepath.Join(root, "xiaohongshu-mcp-darwin-arm64.tar.gz")
	cfg := testConfig(root)
	cfg.DefaultArchive = defaultArchive

	if err := writeTestArchive(defaultArchive, map[string]string{
		cfg.McpBinaryName:   "#!/bin/sh\nexit 0\n",
		cfg.LoginBinaryName: "#!/bin/sh\nexit 0\n",
	}); err != nil {
		t.Fatalf("writeTestArchive() error = %v", err)
	}

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.Install(""); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	status := svc.Status()
	if !status.Installed {
		t.Fatal("status.Installed = false, want true")
	}
	if status.McpBinaryPath != filepath.Join(cfg.BinDir, cfg.McpBinaryName) {
		t.Fatalf("status.McpBinaryPath = %q", status.McpBinaryPath)
	}
	if status.LoginBinaryPath != filepath.Join(cfg.BinDir, cfg.LoginBinaryName) {
		t.Fatalf("status.LoginBinaryPath = %q", status.LoginBinaryPath)
	}
	if status.DefaultArchivePath != defaultArchive {
		t.Fatalf("status.DefaultArchivePath = %q, want %q", status.DefaultArchivePath, defaultArchive)
	}
}

func TestInstallAcceptsWindowsZipArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.DefaultArchive = filepath.Join(root, "xiaohongshu-mcp-windows-amd64.zip")
	cfg.McpBinaryName = "xiaohongshu-mcp-windows-amd64.exe"
	cfg.LoginBinaryName = "xiaohongshu-login-windows-amd64.exe"

	if err := writeTestZipArchive(cfg.DefaultArchive, map[string]string{
		cfg.McpBinaryName:   "binary-mcp",
		cfg.LoginBinaryName: "binary-login",
	}); err != nil {
		t.Fatalf("writeTestZipArchive() error = %v", err)
	}

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.Install(""); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(cfg.BinDir, cfg.McpBinaryName)); err != nil {
		t.Fatalf("mcp binary stat error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.BinDir, cfg.LoginBinaryName)); err != nil {
		t.Fatalf("login binary stat error = %v", err)
	}
}

func TestInstallFailsWhenArchiveIsIncomplete(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := testConfig(root)
	archivePath := filepath.Join(root, "incomplete.tar.gz")

	if err := writeTestArchive(archivePath, map[string]string{
		cfg.McpBinaryName: "#!/bin/sh\nexit 0\n",
	}); err != nil {
		t.Fatalf("writeTestArchive() error = %v", err)
	}

	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	err = svc.Install(archivePath)
	if err == nil {
		t.Fatal("Install() error = nil, want incomplete archive error")
	}
	if !strings.Contains(err.Error(), "archive missing binaries") {
		t.Fatalf("Install() error = %v", err)
	}
}

func TestStartLoginSpawnsProcessAndWritesPid(t *testing.T) {
	root := t.TempDir()
	cfg := testConfig(root)
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	loginScript := "#!/bin/sh\necho login-started\nsleep 5\n"
	if err := os.WriteFile(filepath.Join(cfg.BinDir, cfg.LoginBinaryName), []byte(loginScript), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	pid, err := svc.StartLogin()
	if err != nil {
		t.Fatalf("StartLogin() error = %v", err)
	}
	t.Cleanup(func() {
		process, findErr := os.FindProcess(pid)
		if findErr == nil {
			_ = process.Kill()
		}
	})

	if pid <= 0 {
		t.Fatalf("pid = %d, want > 0", pid)
	}
	if got := svc.readPID(svc.loginPIDPath()); got != pid {
		t.Fatalf("login pid file = %d, want %d", got, pid)
	}
	if status := svc.Status(); status.LoginPid != pid {
		t.Fatalf("status.LoginPid = %d, want %d", status.LoginPid, pid)
	}
	logDeadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(logDeadline) {
		data, readErr := os.ReadFile(filepath.Join(cfg.LogDir, "login.log"))
		if readErr == nil && strings.Contains(string(data), "login-started") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("login log did not contain expected output")
}

func TestPublishReusesSessionAndMapsResponse(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	initializeCalls := 0
	toolCalls := 0
	notificationCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}

		method, _ := payload["method"].(string)
		mu.Lock()
		defer mu.Unlock()

		switch method {
		case "initialize":
			initializeCalls++
			if got := r.Header.Get("Accept"); !strings.Contains(got, "application/json") || !strings.Contains(got, "text/event-stream") {
				t.Fatalf("accept header = %q", got)
			}
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{"serverInfo": map[string]any{"name": "test"}}})
		case "notifications/initialized":
			notificationCalls++
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			toolCalls++
			if got := r.Header.Get("Accept"); !strings.Contains(got, "application/json") || !strings.Contains(got, "text/event-stream") {
				t.Fatalf("accept header = %q", got)
			}
			if got := r.Header.Get(mcpSessionHeader); got != "session-1" {
				t.Fatalf("session header = %q, want %q", got, "session-1")
			}
			params := payload["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			if args["title"] != "title" || args["content"] != "content" {
				t.Fatalf("publish args = %#v", args)
			}
			tags, ok := args["tags"].([]any)
			if !ok || len(tags) != 2 || tags[0] != "#blue" || tags[1] != "#ootd" {
				t.Fatalf("publish tags = %#v", args["tags"])
			}
			writeTestJSON(t, w, map[string]any{
				"result": map[string]any{
					"content": []map[string]any{
						{"text": "publish ok"},
					},
					"structuredContent": map[string]any{
						"noteId": "note-1",
						"postId": "post-1",
					},
				},
			})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}
	cfg := testConfig(root)
	cfg.McpBaseURL = server.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	request := model.PublishRequest{
		Title:   "title",
		Content: "content",
		Tags:    []string{"#blue", "#ootd"},
		Images:  []string{localImagePath},
	}

	first, err := svc.Publish(request)
	if err != nil {
		t.Fatalf("first Publish() error = %v", err)
	}
	second, err := svc.Publish(request)
	if err != nil {
		t.Fatalf("second Publish() error = %v", err)
	}

	if first.Message != "publish ok" || second.Message != "publish ok" {
		t.Fatalf("messages = %q / %q", first.Message, second.Message)
	}
	if first.NoteID != "note-1" || first.PostID != "post-1" {
		t.Fatalf("first publish ids = %#v", first)
	}
	logData, err := os.ReadFile(filepath.Join(cfg.LogDir, "helper.log"))
	if err != nil {
		t.Fatalf("ReadFile(helper.log) error = %v", err)
	}
	if !strings.Contains(string(logData), `publish response parsed`) || !strings.Contains(string(logData), `noteId="note-1"`) {
		t.Fatalf("helper log = %s", string(logData))
	}

	mu.Lock()
	defer mu.Unlock()
	if initializeCalls != 1 {
		t.Fatalf("initializeCalls = %d, want 1", initializeCalls)
	}
	if notificationCalls != 1 {
		t.Fatalf("notificationCalls = %d, want 1", notificationCalls)
	}
	if toolCalls != 2 {
		t.Fatalf("toolCalls = %d, want 2", toolCalls)
	}
}

func TestPublishLogsRawResponseWhenIdsAreMissing(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{"serverInfo": map[string]any{"name": "test"}}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			writeTestJSON(t, w, map[string]any{
				"result": map[string]any{
					"content": []map[string]any{
						{"text": "publish returned without ids"},
					},
				},
			})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}
	cfg := testConfig(root)
	cfg.McpBaseURL = server.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	resp, err := svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Tags:    []string{"#tag"},
		Images:  []string{localImagePath},
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	if resp.Message != "publish returned without ids" {
		t.Fatalf("resp.Message = %q", resp.Message)
	}

	logData, err := os.ReadFile(filepath.Join(cfg.LogDir, "helper.log"))
	if err != nil {
		t.Fatalf("ReadFile(helper.log) error = %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, `publish response missing ids`) || !strings.Contains(logText, `publish returned without ids`) {
		t.Fatalf("helper log = %s", logText)
	}
}

func TestPublishFailsWhenInitializeResponseOmitsSessionHeader(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}
	cfg := testConfig(root)
	cfg.McpBaseURL = server.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	_, err = svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want missing session header error")
	}
	if !strings.Contains(err.Error(), "initialize missing Mcp-Session-Id") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishSurfacesUpstreamError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			writeTestJSON(t, w, map[string]any{"error": map[string]any{"message": "publish failed upstream"}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}
	cfg := testConfig(root)
	cfg.McpBaseURL = server.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	_, err = svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want upstream error")
	}
	if !strings.Contains(err.Error(), "publish failed upstream") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishDownloadsRemoteImagesIntoLocalPreparedFiles(t *testing.T) {
	t.Parallel()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writePNG(t, w, 256, 128, color.NRGBA{R: 60, G: 120, B: 220, A: 255})
	}))
	defer imageServer.Close()

	var publishedImages []string
	var preparedImageExistsDuringPublish bool
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			params := payload["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			publishedImages = toStringSlice(t, args["images"])
			if len(publishedImages) == 1 {
				_, err := os.Stat(publishedImages[0])
				preparedImageExistsDuringPublish = err == nil
			}
			writeTestJSON(t, w, map[string]any{"result": map[string]any{
				"content": []map[string]any{{"text": "publish ok"}},
			}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	if _, err := svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{imageServer.URL + "/hero.png"},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(publishedImages) != 1 {
		t.Fatalf("publishedImages = %#v", publishedImages)
	}
	if strings.HasPrefix(publishedImages[0], "http://") || strings.HasPrefix(publishedImages[0], "https://") {
		t.Fatalf("published image path still remote: %q", publishedImages[0])
	}
	if !strings.HasPrefix(publishedImages[0], cfg.TmpDir) {
		t.Fatalf("published image path = %q, want under %q", publishedImages[0], cfg.TmpDir)
	}
	if !preparedImageExistsDuringPublish {
		t.Fatal("prepared image did not exist during MCP publish")
	}
	if _, err := os.Stat(publishedImages[0]); !os.IsNotExist(err) {
		t.Fatalf("prepared image stat err = %v, want not exist after cleanup", err)
	}
}

func TestPublishTreatsFailureTextAsUpstreamError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			writeTestJSON(t, w, map[string]any{
				"result": map[string]any{
					"content": []map[string]any{
						{"text": "发布失败: 标题长度超过限制"},
					},
				},
			})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer server.Close()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}
	cfg := testConfig(root)
	cfg.McpBaseURL = server.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	_, err = svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want upstream business failure")
	}
	if !strings.Contains(err.Error(), "mcp publish failed: 发布失败: 标题长度超过限制") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishNormalizesOversizedImageToMax2K(t *testing.T) {
	t.Parallel()

	var encoded []byte
	func() {
		img := image.NewNRGBA(image.Rect(0, 0, 4096, 1024))
		fillImage(img, color.NRGBA{R: 20, G: 30, B: 40, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("png.Encode() error = %v", err)
		}
		encoded = buf.Bytes()
	}()

	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if _, err := w.Write(encoded); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}))
	defer imageServer.Close()

	var publishedImages []string
	var publishedWidth int
	var publishedHeight int
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			params := payload["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			publishedImages = toStringSlice(t, args["images"])
			if len(publishedImages) == 1 {
				file, err := os.Open(publishedImages[0])
				if err != nil {
					t.Fatalf("Open(prepared image) error = %v", err)
				}
				defer file.Close()
				cfg, _, err := image.DecodeConfig(file)
				if err != nil {
					t.Fatalf("DecodeConfig() error = %v", err)
				}
				publishedWidth = cfg.Width
				publishedHeight = cfg.Height
			}
			writeTestJSON(t, w, map[string]any{"result": map[string]any{
				"content": []map[string]any{{"text": "publish ok"}},
			}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	if _, err := svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{imageServer.URL + "/wide.png"},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(publishedImages) != 1 {
		t.Fatalf("publishedImages = %#v", publishedImages)
	}
	if publishedWidth != 2048 || publishedHeight != 512 {
		t.Fatalf("prepared image size = %dx%d, want 2048x512", publishedWidth, publishedHeight)
	}
}

func TestPublishReturnsImageSpecificErrorOnDownloadFailure(t *testing.T) {
	t.Parallel()

	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			t.Fatal("tools/call should not execute when image download fails")
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	root := t.TempDir()
	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	_, err = svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{"http://127.0.0.1:1/missing.png"},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want image-specific download failure")
	}
	if !strings.Contains(err.Error(), "image 1") || !strings.Contains(err.Error(), "http://127.0.0.1:1/missing.png") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func TestPublishAcceptsLocalImagePathAndPreservesSmallTransparentPNG(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "alpha.png")
	if err := os.WriteFile(localImagePath, transparentPNG(t, 640, 320), 0o644); err != nil {
		t.Fatalf("WriteFile(alpha.png) error = %v", err)
	}

	var publishedImages []string
	var publishedWidth int
	var publishedHeight int
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			params := payload["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			publishedImages = toStringSlice(t, args["images"])
			if len(publishedImages) == 1 {
				file, err := os.Open(publishedImages[0])
				if err != nil {
					t.Fatalf("Open(prepared image) error = %v", err)
				}
				defer file.Close()
				cfg, format, err := image.DecodeConfig(file)
				if err != nil {
					t.Fatalf("DecodeConfig() error = %v", err)
				}
				publishedWidth = cfg.Width
				publishedHeight = cfg.Height
				if format != "png" {
					t.Fatalf("prepared format = %q, want png", format)
				}
			}
			writeTestJSON(t, w, map[string]any{"result": map[string]any{
				"content": []map[string]any{{"text": "publish ok"}},
			}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	if _, err := svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(publishedImages) != 1 {
		t.Fatalf("publishedImages = %#v", publishedImages)
	}
	if publishedImages[0] == localImagePath {
		t.Fatalf("published image path should be prepared copy, got original %q", publishedImages[0])
	}
	if publishedWidth != 640 || publishedHeight != 320 {
		t.Fatalf("prepared image size = %dx%d, want 640x320", publishedWidth, publishedHeight)
	}
}

func TestPublishCompressesOpaquePreparedImageToTwoMBOrLess(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "noise.png")
	if err := os.WriteFile(localImagePath, noisyOpaquePNG(t, 3200, 3200), 0o644); err != nil {
		t.Fatalf("WriteFile(noise.png) error = %v", err)
	}

	var preparedSize int64
	var publishedWidth int
	var publishedHeight int
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			params := payload["params"].(map[string]any)
			args := params["arguments"].(map[string]any)
			images := toStringSlice(t, args["images"])
			if len(images) != 1 {
				t.Fatalf("published images = %#v", images)
			}
			info, err := os.Stat(images[0])
			if err != nil {
				t.Fatalf("Stat(prepared image) error = %v", err)
			}
			preparedSize = info.Size()
			file, err := os.Open(images[0])
			if err != nil {
				t.Fatalf("Open(prepared image) error = %v", err)
			}
			defer file.Close()
			cfg, _, err := image.DecodeConfig(file)
			if err != nil {
				t.Fatalf("DecodeConfig() error = %v", err)
			}
			publishedWidth = cfg.Width
			publishedHeight = cfg.Height
			writeTestJSON(t, w, map[string]any{"result": map[string]any{
				"content": []map[string]any{{"text": "publish ok"}},
			}})
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	if _, err := svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if preparedSize <= 0 {
		t.Fatalf("preparedSize = %d", preparedSize)
	}
	if preparedSize > 2*1024*1024 {
		t.Fatalf("prepared image size = %d, want <= %d", preparedSize, 2*1024*1024)
	}
	if publishedWidth > 2048 || publishedHeight > 2048 {
		t.Fatalf("prepared image size = %dx%d, want longest edge <= 2048", publishedWidth, publishedHeight)
	}
}

func TestPublishReturnsClearErrorWhenTransparentImageCannotFitTwoMB(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	localImagePath := filepath.Join(root, "noise-alpha.png")
	if err := os.WriteFile(localImagePath, noisyTransparentPNG(t, 3200, 3200), 0o644); err != nil {
		t.Fatalf("WriteFile(noise-alpha.png) error = %v", err)
	}

	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("Decode() error = %v", err)
		}
		method, _ := payload["method"].(string)
		switch method {
		case "initialize":
			w.Header().Set(mcpSessionHeader, "session-1")
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "notifications/initialized":
			writeTestJSON(t, w, map[string]any{"result": map[string]any{}})
		case "tools/call":
			t.Fatal("tools/call should not execute when compression cannot meet 2MB limit")
		default:
			t.Fatalf("unexpected method %q", method)
		}
	}))
	defer mcpServer.Close()

	cfg := testConfig(root)
	cfg.McpBaseURL = mcpServer.URL
	svc, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.writePID(svc.mcpPIDPath(), os.Getpid()); err != nil {
		t.Fatalf("writePID() error = %v", err)
	}

	_, err = svc.Publish(model.PublishRequest{
		Title:   "title",
		Content: "content",
		Images:  []string{localImagePath},
	})
	if err == nil {
		t.Fatal("Publish() error = nil, want compression limit failure")
	}
	if !strings.Contains(err.Error(), "exceeds 2097152 bytes") {
		t.Fatalf("Publish() error = %v", err)
	}
}

func testConfig(root string) config.Config {
	return config.Config{
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  filepath.Join(root, "missing.tar.gz"),
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      "http://127.0.0.1:18060/mcp",
		McpPort:         "18060",
	}
}

func writeTestArchive(path string, files map[string]string) error {
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

func writeTestZipArchive(path string, files map[string]string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	defer zw.Close()

	for name, body := range files {
		entry, err := zw.Create(name)
		if err != nil {
			return err
		}
		if _, err := entry.Write([]byte(body)); err != nil {
			return err
		}
	}
	return nil
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
}

func writePNG(t *testing.T, w http.ResponseWriter, width, height int, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	fillImage(img, fill)
	w.Header().Set("Content-Type", "image/png")
	if err := png.Encode(w, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
}

func transparentPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			alpha := uint8(255)
			if x < img.Bounds().Dx()/2 {
				alpha = 180
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 40, G: 90, B: 180, A: alpha})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func opaquePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	fillImage(img, color.NRGBA{R: 50, G: 100, B: 150, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func noisyOpaquePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r := uint8((x*37 + y*17) % 256)
			g := uint8((x*13 + y*29) % 256)
			b := uint8((x*53 + y*7) % 256)
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func noisyTransparentPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r := uint8((x*37 + y*17) % 256)
			g := uint8((x*13 + y*29) % 256)
			b := uint8((x*53 + y*7) % 256)
			a := uint8((x*19 + y*23) % 256)
			if a == 255 {
				a = 254
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func fillImage(img *image.NRGBA, fill color.NRGBA) {
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
}

func toStringSlice(t *testing.T, value any) []string {
	t.Helper()
	raw, ok := value.([]any)
	if !ok {
		t.Fatalf("value %#v is not []any", value)
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("item %#v is not string", item)
		}
		out = append(out, text)
	}
	return out
}
