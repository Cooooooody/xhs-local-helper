package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/config"
	"github.com/liuhaotian/xhs-local-helper/internal/model"
)

func TestInstallRejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/install", bytes.NewBufferString("{"))
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["message"] != "invalid json body" {
		t.Fatalf("message = %v, want %q", response["message"], "invalid json body")
	}
	if response["code"] != float64(model.CodeBadRequest) {
		t.Fatalf("code = %v, want %d", response["code"], model.CodeBadRequest)
	}
	if response["success"] != false {
		t.Fatalf("success = %v, want false", response["success"])
	}
}

func TestInstallAllowsEmptyBodyAndFallsBackToDefaultArchive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	defaultArchive := filepath.Join(root, "missing.tar.gz")
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  defaultArchive,
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      "http://127.0.0.1:18060/mcp",
		McpPort:         "18060",
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/install", http.NoBody)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	want := "archive not found: " + defaultArchive
	if response["message"] != want {
		t.Fatalf("message = %v, want %q", response["message"], want)
	}
	if response["code"] != float64(model.CodeInstallFailed) {
		t.Fatalf("code = %v, want %d", response["code"], model.CodeInstallFailed)
	}
}

func TestPublishRejectsMissingImagesWithCode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewBufferString(`{"title":"t","content":"c","images":[]}`))
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["code"] != float64(model.CodeImagesEmpty) {
		t.Fatalf("code = %v, want %d", response["code"], model.CodeImagesEmpty)
	}
	if response["message"] != "images are required" {
		t.Fatalf("message = %v, want %q", response["message"], "images are required")
	}
}

func TestClassifyPublishErrorMapsUpstreamBusinessFailure(t *testing.T) {
	t.Parallel()

	code := classifyPublishError(errors.New("mcp publish failed: 发布失败: 标题长度超过限制"))
	if code != model.CodePublishUpstream {
		t.Fatalf("code = %d, want %d", code, model.CodePublishUpstream)
	}
}

func TestStatusAllowsLoopbackOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "http://127.0.0.1:8000")
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:8000" {
		t.Fatalf("allow origin = %q", got)
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if response["code"] != float64(model.CodeOK) {
		t.Fatalf("code = %v, want %d", response["code"], model.CodeOK)
	}
	if response["success"] != true {
		t.Fatalf("success = %v, want true", response["success"])
	}
}

func TestStatusAllowsMusegateOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "https://musegate.tech")
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://musegate.tech" {
		t.Fatalf("allow origin = %q", got)
	}
}

func TestStatusAllowsConrainOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	req.Header.Set("Origin", "https://aigc-dev.conrain.cn")
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://aigc-dev.conrain.cn" {
		t.Fatalf("allow origin = %q", got)
	}
}

func TestPublishPreflightAllowsLoopbackOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/publish", nil)
	req.Header.Set("Origin", "http://localhost:8000")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:8000" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !bytes.Contains([]byte(got), []byte("OPTIONS")) {
		t.Fatalf("allow methods = %q", got)
	}
}

func TestPublishPreflightRejectsNonMusegateNonLoopbackOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/publish", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("allow origin = %q, want empty", got)
	}
}

func TestPublishPreflightAllowsConrainOrigin(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
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

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/publish", nil)
	req.Header.Set("Origin", "https://aigc-dev.conrain.cn")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	recorder := httptest.NewRecorder()

	server.httpServer.Handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://aigc-dev.conrain.cn" {
		t.Fatalf("allow origin = %q", got)
	}
}

func TestHelperServerEndToEndFlowWithFakeBinaries(t *testing.T) {
	root := t.TempDir()
	archivePath := filepath.Join(root, "fake-xhs.tar.gz")
	mcpPort := freeTCPPort(t)
	cfg := config.Config{
		ListenAddr:      "127.0.0.1:0",
		AppDir:          root,
		BinDir:          filepath.Join(root, "bin"),
		RunDir:          filepath.Join(root, "run"),
		LogDir:          filepath.Join(root, "logs"),
		TmpDir:          filepath.Join(root, "tmp"),
		DefaultArchive:  archivePath,
		McpBinaryName:   "xiaohongshu-mcp-darwin-arm64",
		LoginBinaryName: "xiaohongshu-login-darwin-arm64",
		McpBaseURL:      "http://127.0.0.1:" + mcpPort + "/mcp",
		McpPort:         mcpPort,
	}

	if err := writeArchive(archivePath, map[string]string{
		cfg.LoginBinaryName: `#!/bin/sh
echo login-started
sleep 5
`,
		cfg.McpBinaryName: `#!/bin/sh
PORT=18060
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-port" ]; then
    PORT="${2#:}"
    shift 2
    continue
  fi
  shift
done
exec python3 - "$PORT" <<'PY'
import json
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

port = int(sys.argv[1])

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b"ok")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", "0"))
        payload = json.loads(self.rfile.read(length) or b"{}")
        method = payload.get("method")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        if method == "initialize":
            self.send_header("Mcp-Session-Id", "session-1")
            self.end_headers()
            self.wfile.write(json.dumps({"result": {"serverInfo": {"name": "fake"}}}).encode())
        elif method == "notifications/initialized":
            self.end_headers()
            self.wfile.write(b'{"result":{}}')
        elif method == "tools/call":
            self.end_headers()
            self.wfile.write(json.dumps({
                "result": {
                    "content": [{"text": "publish ok"}],
                    "structuredContent": {"noteId": "note-1", "postId": "post-1"}
                }
            }).encode())
        else:
            self.end_headers()
            self.wfile.write(b'{"error":{"message":"unexpected method"}}')

    def log_message(self, format, *args):
        pass

HTTPServer(("127.0.0.1", port), Handler).serve_forever()
PY
`,
	}); err != nil {
		t.Fatalf("writeArchive() error = %v", err)
	}

	server, err := NewServer(cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()
	defer server.httpServer.Close()

	go func() {
		_ = server.httpServer.Serve(listener)
	}()

	baseURL := "http://" + listener.Addr().String()
	client := &http.Client{Timeout: 5 * time.Second}
	localImagePath := filepath.Join(root, "publish.png")
	if err := os.WriteFile(localImagePath, opaquePNG(t, 320, 160), 0o644); err != nil {
		t.Fatalf("WriteFile(publish.png) error = %v", err)
	}

	postJSON(t, client, baseURL+"/install", map[string]any{}, http.StatusOK)
	loginResp := postJSON(t, client, baseURL+"/login", map[string]any{}, http.StatusOK)
	publishResp := postJSON(t, client, baseURL+"/publish", map[string]any{
		"title":   "title",
		"content": "content",
		"images":  []string{localImagePath},
	}, http.StatusOK)

	loginBody := decodeBody(t, loginResp.Body)
	if loginBody["code"] != float64(model.CodeOK) || loginBody["success"] != true {
		t.Fatalf("login body = %#v", loginBody)
	}
	if loginBody["pid"] == nil {
		t.Fatalf("login response missing pid: %#v", loginBody)
	}

	publishBody := decodeBody(t, publishResp.Body)
	if publishBody["code"] != float64(model.CodeOK) || publishBody["success"] != true {
		t.Fatalf("publish body = %#v", publishBody)
	}
	if publishBody["message"] != "publish ok" {
		t.Fatalf("publish message = %v", publishBody["message"])
	}
	if publishBody["noteId"] != "note-1" || publishBody["postId"] != "post-1" {
		t.Fatalf("publish body = %#v", publishBody)
	}

	statusResp, err := client.Get(baseURL + "/status")
	if err != nil {
		t.Fatalf("GET /status error = %v", err)
	}
	defer statusResp.Body.Close()
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status status = %d", statusResp.StatusCode)
	}

	statusBody := decodeBody(t, statusResp.Body)
	if statusBody["code"] != float64(model.CodeOK) || statusBody["success"] != true {
		t.Fatalf("status body = %#v", statusBody)
	}
	if statusBody["installed"] != true {
		t.Fatalf("status installed = %v", statusBody["installed"])
	}
	if statusBody["mcpRunning"] != true {
		t.Fatalf("status mcpRunning = %v", statusBody["mcpRunning"])
	}
}

func postJSON(t *testing.T, client *http.Client, url string, body any, wantStatus int) *http.Response {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s error = %v", url, err)
	}
	if resp.StatusCode != wantStatus {
		defer resp.Body.Close()
		payload, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST %s status = %d, want %d, body=%s", url, resp.StatusCode, wantStatus, payload)
	}
	return resp
}

func decodeBody(t *testing.T, body io.ReadCloser) map[string]any {
	t.Helper()
	defer body.Close()
	var payload map[string]any
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return payload
}

func opaquePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 50, G: 100, B: 150, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
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

	for name, content := range files {
		data := []byte(content)
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(data)),
		}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer listener.Close()
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	return port
}
