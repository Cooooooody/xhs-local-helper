package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/liuhaotian/xhs-local-helper/internal/config"
	"github.com/liuhaotian/xhs-local-helper/internal/helper"
	"github.com/liuhaotian/xhs-local-helper/internal/model"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg config.Config) (*Server, error) {
	svc, err := helper.New(cfg)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, model.CodeMethodNotAllowed, "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, svc.Status())
	})
	mux.HandleFunc("/install", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, model.CodeMethodNotAllowed, "method not allowed")
			return
		}
		var req model.InstallRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, model.CodeBadRequest, "invalid json body")
			return
		}
		if err := svc.Install(req.ArchivePath); err != nil {
			writeError(w, http.StatusBadRequest, model.CodeInstallFailed, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, model.ActionResponse{
			Success: true,
			Code:    model.CodeOK,
			Message: "install completed",
		})
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, model.CodeMethodNotAllowed, "method not allowed")
			return
		}
		pid, err := svc.StartLogin()
		if err != nil {
			writeError(w, http.StatusBadRequest, model.CodeLoginFailed, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, model.ActionResponse{
			Success: true,
			Code:    model.CodeOK,
			Message: "login process started",
			Pid:     pid,
		})
	})
	mux.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, model.CodeMethodNotAllowed, "method not allowed")
			return
		}
		var req model.PublishRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, model.CodeBadRequest, "invalid json body")
			return
		}
		if code, message := validatePublishRequest(req); code != 0 {
			writeError(w, http.StatusBadRequest, code, message)
			return
		}
		resp, err := svc.Publish(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, classifyPublishError(err), err.Error())
			return
		}
		resp.Success = true
		resp.Code = model.CodeOK
		writeJSON(w, http.StatusOK, resp)
	})

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: withCORS(mux),
		},
	}, nil
}

func (s *Server) ListenAndServe() error {
	if s == nil || s.httpServer == nil {
		return errors.New("server is nil")
	}
	return s.httpServer.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code int, message string) {
	writeJSON(w, status, model.ErrorResponse{
		Success: false,
		Code:    code,
		Message: message,
	})
}

func validatePublishRequest(req model.PublishRequest) (int, string) {
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return model.CodeBadRequest, "title and content are required"
	}
	if len(req.Images) == 0 {
		return model.CodeImagesEmpty, "images are required"
	}
	for _, image := range req.Images {
		if strings.TrimSpace(image) == "" {
			return model.CodeImageSourceInvalid, "images contain empty value"
		}
	}
	return 0, ""
}

func classifyPublishError(err error) int {
	if err == nil {
		return model.CodeInternal
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "invalid source"), strings.Contains(message, "local file not found"):
		return model.CodeImageSourceInvalid
	case strings.Contains(message, "download from"), strings.Contains(message, "http status"):
		return model.CodeImageDownloadFail
	case strings.Contains(message, "normalize"), strings.Contains(message, "exceeds"):
		return model.CodeImagePrepareFail
	case strings.HasPrefix(message, "mcp binary not installed"):
		return model.CodeMcpRuntimeMissing
	case strings.Contains(message, "mcp did not become ready"):
		return model.CodeMcpUnavailable
	case strings.Contains(message, "initialize missing Mcp-Session-Id"):
		return model.CodeMcpSessionFailed
	case strings.HasPrefix(message, "mcp publish failed:"):
		return model.CodePublishUpstream
	default:
		return model.CodeInternal
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := allowedOrigin(r.Header.Get("Origin")); origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Musegate-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOrigin(origin string) string {
	if origin == "" {
		return ""
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return ""
	}
	host := parsed.Hostname()
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return origin
	}
	if parsed.Scheme == "https" && (isMusegateHost(host) || isConrainHost(host)) {
		return origin
	}
	return ""
}

func isMusegateHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "musegate.tech" {
		return true
	}
	if strings.HasSuffix(host, ".musegate.tech") {
		return true
	}
	return false
}

func isConrainHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "conrain.cn" {
		return true
	}
	if strings.HasSuffix(host, ".conrain.cn") {
		return true
	}
	return false
}
