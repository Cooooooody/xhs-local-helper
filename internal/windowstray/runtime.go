package windowstray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/liuhaotian/xhs-local-helper/internal/windowsbundle"
)

const PublishPageURL = "https://musegate.tech/#/text2img/auto-generation"

type RuntimeConfig struct {
	HomeDir      string
	LocalAppData string
	HelperPath   string
	ArchivePath  string
	BaseURL      string
	HTTPClient   *http.Client
}

func DefaultRuntimeConfig(homeDir, localAppData, helperPath, archivePath, baseURL string) RuntimeConfig {
	return RuntimeConfig{
		HomeDir:      homeDir,
		LocalAppData: localAppData,
		HelperPath:   helperPath,
		ArchivePath:  archivePath,
		BaseURL:      baseURL,
		HTTPClient:   &http.Client{Timeout: 3 * time.Second},
	}
}

func (c RuntimeConfig) Controller() Controller {
	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	appDir := filepath.Join(c.LocalAppData, "XhsLocalHelper")
	return Controller{
		StatusCheck: func() bool {
			resp, err := client.Get(c.BaseURL + "/status")
			if err != nil {
				return false
			}
			defer resp.Body.Close()
			return resp.StatusCode == http.StatusOK
		},
		StartHelper: func() error {
			cmd := exec.Command(c.HelperPath)
			cmd.Env = os.Environ()
			return cmd.Start()
		},
		InstallRuntime: func() error {
			payload, err := json.Marshal(map[string]string{"archivePath": c.ArchivePath})
			if err != nil {
				return err
			}

			var lastErr error
			for attempt := 0; attempt < 10; attempt++ {
				resp, err := client.Post(c.BaseURL+"/install", "application/json", bytes.NewReader(payload))
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode == http.StatusOK {
						return nil
					}
					lastErr = fmt.Errorf("install request failed: status=%d", resp.StatusCode)
				} else {
					lastErr = err
				}
				time.Sleep(250 * time.Millisecond)
			}
			return lastErr
		},
		CookiePaths: []string{
			filepath.Join(appDir, "cookies.json"),
			filepath.Join(appDir, "cookies.txt"),
		},
		ResetState: func() error {
			for _, path := range []string{
				filepath.Join(appDir, "run", "mcp.pid"),
				filepath.Join(appDir, "run", "login.pid"),
			} {
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		},
		StopAll: func() error {
			for _, name := range []string{
				windowsbundle.HelperBinaryName,
				windowsbundle.McpBinaryName,
				windowsbundle.LoginBinaryName,
			} {
				cmd := exec.Command("taskkill", "/IM", name, "/F")
				_ = cmd.Run()
			}
			return nil
		},
	}
}
