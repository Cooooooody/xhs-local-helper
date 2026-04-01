package helper

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/config"
	"github.com/liuhaotian/xhs-local-helper/internal/model"
)

const (
	mcpSessionHeader  = "Mcp-Session-Id"
	mcpRequestTimeout = 3 * time.Minute
)

type Service struct {
	cfg        config.Config
	httpClient *http.Client

	mu        sync.Mutex
	sessionID string
}

func New(cfg config.Config) (*Service, error) {
	svc := &Service{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: mcpRequestTimeout},
	}
	if err := svc.ensureDirs(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *Service) ensureDirs() error {
	for _, dir := range []string{s.cfg.AppDir, s.cfg.BinDir, s.cfg.RunDir, s.cfg.LogDir, s.cfg.TmpDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func (s *Service) Status() model.StatusResponse {
	mcpPid := s.readPID(s.mcpPIDPath())
	loginPid := s.readPID(s.loginPIDPath())
	mcpRunning := processExists(mcpPid)
	loginRunning := processExists(loginPid)

	return model.StatusResponse{
		Success:            true,
		Code:               model.CodeOK,
		Installed:          fileExists(s.mcpBinaryPath()) && fileExists(s.loginBinaryPath()),
		McpRunning:         mcpRunning,
		LoggedIn:           fileExists(s.cookiesPath()),
		AppDir:             s.cfg.AppDir,
		McpBaseURL:         s.cfg.McpBaseURL,
		McpBinaryPath:      s.mcpBinaryPath(),
		LoginBinaryPath:    s.loginBinaryPath(),
		DefaultArchivePath: s.cfg.DefaultArchive,
		McpPid:             pidOrZero(mcpRunning, mcpPid),
		LoginPid:           pidOrZero(loginRunning, loginPid),
	}
}

func (s *Service) Install(archivePath string) error {
	if archivePath == "" {
		archivePath = s.cfg.DefaultArchive
	}
	if !fileExists(archivePath) {
		return fmt.Errorf("archive not found: %s", archivePath)
	}

	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	if strings.HasSuffix(strings.ToLower(archivePath), ".zip") {
		return s.installFromZipArchive(archivePath)
	}

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip reader: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	var extracted int
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		name := filepath.Base(header.Name)
		switch name {
		case s.cfg.McpBinaryName, s.cfg.LoginBinaryName:
			target := filepath.Join(s.cfg.BinDir, name)
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

func (s *Service) installFromZipArchive(archivePath string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open zip archive: %w", err)
	}
	defer reader.Close()

	var extracted int
	for _, file := range reader.File {
		name := filepath.Base(file.Name)
		switch name {
		case s.cfg.McpBinaryName, s.cfg.LoginBinaryName:
			rc, err := file.Open()
			if err != nil {
				return fmt.Errorf("open zip entry %s: %w", file.Name, err)
			}
			target := filepath.Join(s.cfg.BinDir, name)
			writeErr := writeExecutable(target, rc)
			closeErr := rc.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return fmt.Errorf("close zip entry %s: %w", file.Name, closeErr)
			}
			extracted++
		}
	}
	if extracted < 2 {
		return fmt.Errorf("archive missing binaries, extracted=%d", extracted)
	}
	return nil
}

func (s *Service) StartLogin() (int, error) {
	if !fileExists(s.loginBinaryPath()) {
		return 0, fmt.Errorf("login binary not installed: %s", s.loginBinaryPath())
	}
	cmd := exec.Command(s.loginBinaryPath())
	cmd.Dir = s.cfg.AppDir
	logFile, err := os.OpenFile(filepath.Join(s.cfg.LogDir, "login.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, fmt.Errorf("open login log: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return 0, fmt.Errorf("start login binary: %w", err)
	}
	if err := s.writePID(s.loginPIDPath(), cmd.Process.Pid); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func (s *Service) Publish(req model.PublishRequest) (model.PublishResponse, error) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" || len(req.Images) == 0 {
		return model.PublishResponse{}, fmt.Errorf("title/content/images are required")
	}
	if err := s.ensureMcpRunning(); err != nil {
		return model.PublishResponse{}, err
	}
	if err := s.ensureSession(); err != nil {
		return model.PublishResponse{}, err
	}
	preparedDir, preparedImages, err := s.preparePublishImages(req.Images)
	if err != nil {
		return model.PublishResponse{}, err
	}
	if preparedDir != "" {
		defer func() {
			if err := os.RemoveAll(preparedDir); err != nil {
				s.writeHelperLog("publish temp cleanup failed dir=%q error=%v", preparedDir, err)
			}
		}()
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("publish-%d", time.Now().UnixMilli()),
		"method":  "tools/call",
		"params": map[string]any{
			"name": "publish_content",
			"arguments": map[string]any{
				"title":   req.Title,
				"content": req.Content,
				"tags":    tags,
				"images":  preparedImages,
			},
		},
	}

	body, _, err := s.rawPostMcpWithSession(payload, s.sessionID)
	if err != nil {
		return model.PublishResponse{}, err
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return model.PublishResponse{}, fmt.Errorf("decode mcp publish response: %w", err)
	}
	if errObj, ok := root["error"].(map[string]any); ok && len(errObj) > 0 {
		s.writeHelperLog(
			"publish upstream error title=%q images=%d tags=%d error=%v raw=%s",
			req.Title,
			len(req.Images),
			len(req.Tags),
			errObj["message"],
			truncateForLog(body, 1200),
		)
		return model.PublishResponse{}, fmt.Errorf("mcp publish failed: %v", errObj["message"])
	}

	resp := model.PublishResponse{Message: "publish requested"}
	result, _ := root["result"].(map[string]any)
	content, _ := result["content"].([]any)
	for _, item := range content {
		entry, _ := item.(map[string]any)
		text, _ := entry["text"].(string)
		if strings.TrimSpace(text) != "" {
			resp.Message = text
		}
	}
	if publishMessageIndicatesFailure(resp.Message) {
		s.writeHelperLog(
			"publish upstream business failure title=%q images=%d tags=%d message=%q raw=%s",
			req.Title,
			len(req.Images),
			len(req.Tags),
			resp.Message,
			truncateForLog(body, 1200),
		)
		return model.PublishResponse{}, fmt.Errorf("mcp publish failed: %s", resp.Message)
	}
	structured, _ := result["structuredContent"].(map[string]any)
	if noteID, ok := structured["noteId"].(string); ok {
		resp.NoteID = noteID
	}
	if postID, ok := structured["postId"].(string); ok {
		resp.PostID = postID
	}
	if resp.NoteID == "" && resp.PostID == "" {
		s.writeHelperLog(
			"publish response missing ids title=%q images=%d tags=%d message=%q raw=%s",
			req.Title,
			len(req.Images),
			len(req.Tags),
			resp.Message,
			truncateForLog(body, 1200),
		)
	} else {
		s.writeHelperLog(
			"publish response parsed title=%q images=%d tags=%d noteId=%q postId=%q message=%q",
			req.Title,
			len(req.Images),
			len(req.Tags),
			resp.NoteID,
			resp.PostID,
			resp.Message,
		)
	}
	return resp, nil
}

func publishMessageIndicatesFailure(message string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(message))
	return strings.HasPrefix(trimmed, "发布失败") ||
		strings.HasPrefix(trimmed, "publish failed") ||
		strings.HasPrefix(trimmed, "publish error")
}

func (s *Service) ensureMcpRunning() error {
	if processExists(s.readPID(s.mcpPIDPath())) {
		return nil
	}
	if !fileExists(s.mcpBinaryPath()) {
		return fmt.Errorf("mcp binary not installed: %s", s.mcpBinaryPath())
	}

	cmd := exec.Command(s.mcpBinaryPath(), "-headless=true", "-port", ":"+s.cfg.McpPort)
	cmd.Dir = s.cfg.AppDir
	logFile, err := os.OpenFile(filepath.Join(s.cfg.LogDir, "mcp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open mcp log: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("start mcp binary: %w", err)
	}
	if err := s.writePID(s.mcpPIDPath(), cmd.Process.Pid); err != nil {
		return err
	}

	deadline := time.Now().Add(8 * time.Second)
	if err := s.waitForMcpReady(time.Until(deadline)); err != nil {
		return err
	}
	return nil
}

func (s *Service) waitForMcpReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := s.httpClient.Get(s.cfg.McpBaseURL)
		if err == nil && resp != nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("mcp did not become ready within %s", timeout)
}

func (s *Service) ensureSession() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.sessionID != "" {
		return nil
	}

	initReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("init-%d", time.Now().UnixMilli()),
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"clientInfo": map[string]any{
				"name":    "xhs-local-helper",
				"version": "0.1.0",
			},
		},
	}

	body, headers, err := s.rawPostMcp(initReq)
	if err != nil {
		return err
	}
	sessionID := headers.Get(mcpSessionHeader)
	if sessionID == "" {
		return fmt.Errorf("initialize missing %s, body=%s", mcpSessionHeader, body)
	}
	s.sessionID = sessionID

	_, _, err = s.rawPostMcpWithSession(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}, s.sessionID)
	return err
}

func (s *Service) postMcp(payload any, out any) error {
	body, _, err := s.rawPostMcpWithSession(payload, s.sessionID)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal([]byte(body), out)
}

func (s *Service) rawPostMcp(payload any) (string, http.Header, error) {
	return s.rawPostMcpWithSession(payload, "")
}

func (s *Service) rawPostMcpWithSession(payload any, sessionID string) (string, http.Header, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", nil, fmt.Errorf("marshal mcp request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.cfg.McpBaseURL, bytes.NewReader(data))
	if err != nil {
		return "", nil, fmt.Errorf("new mcp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if sessionID != "" {
		req.Header.Set(mcpSessionHeader, sessionID)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("call mcp: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("read mcp response: %w", err)
	}
	return string(body), resp.Header.Clone(), nil
}

func (s *Service) mcpBinaryPath() string   { return filepath.Join(s.cfg.BinDir, s.cfg.McpBinaryName) }
func (s *Service) loginBinaryPath() string { return filepath.Join(s.cfg.BinDir, s.cfg.LoginBinaryName) }
func (s *Service) mcpPIDPath() string      { return filepath.Join(s.cfg.RunDir, "mcp.pid") }
func (s *Service) loginPIDPath() string    { return filepath.Join(s.cfg.RunDir, "login.pid") }
func (s *Service) cookiesPath() string     { return filepath.Join(s.cfg.AppDir, "cookies.json") }
func (s *Service) helperLogPath() string   { return filepath.Join(s.cfg.LogDir, "helper.log") }

func (s *Service) writePID(path string, pid int) error {
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

func (s *Service) readPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return process.Signal(syscall.Signal(0)) == nil
}

func pidOrZero(ok bool, pid int) int {
	if ok {
		return pid
	}
	return 0
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (s *Service) writeHelperLog(format string, args ...any) {
	file, err := os.OpenFile(s.helperLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
}

func truncateForLog(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit] + "...(truncated)"
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
