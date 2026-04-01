package windowstray

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRuntimeControllerInstallRuntimeRetriesUntilHelperReady(t *testing.T) {
	t.Parallel()

	var installCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/install" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		installCalls++
		if installCalls == 1 {
			http.Error(w, "warming up", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	runtimeCfg := DefaultRuntimeConfig(
		t.TempDir(),
		t.TempDir(),
		"C:\\bundle\\xhs-local-helper-windows-amd64.exe",
		"C:\\bundle\\xiaohongshu-mcp-windows-amd64.zip",
		server.URL,
	)

	if err := runtimeCfg.Controller().InstallRuntime(); err != nil {
		t.Fatalf("InstallRuntime() error = %v", err)
	}
	if installCalls != 2 {
		t.Fatalf("installCalls = %d, want 2", installCalls)
	}
}
