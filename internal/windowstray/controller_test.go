package windowstray

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureHelperStartedStartsWhenStatusUnavailable(t *testing.T) {
	t.Parallel()

	var starts int
	controller := Controller{
		StatusCheck: func() bool { return false },
		StartHelper: func() error {
			starts++
			return nil
		},
		InstallRuntime: func() error { return nil },
	}

	outcome, err := controller.EnsureHelperStarted()
	if err != nil {
		t.Fatalf("EnsureHelperStarted() error = %v", err)
	}
	if outcome != LaunchOutcomeStarted {
		t.Fatalf("outcome = %q, want %q", outcome, LaunchOutcomeStarted)
	}
	if starts != 1 {
		t.Fatalf("starts = %d, want 1", starts)
	}
}

func TestEnsureHelperStartedReusesExistingHelper(t *testing.T) {
	t.Parallel()

	var starts int
	controller := Controller{
		StatusCheck:    func() bool { return true },
		StartHelper:    func() error { starts++; return nil },
		InstallRuntime: func() error { return nil },
	}

	outcome, err := controller.EnsureHelperStarted()
	if err != nil {
		t.Fatalf("EnsureHelperStarted() error = %v", err)
	}
	if outcome != LaunchOutcomeAlreadyRunning {
		t.Fatalf("outcome = %q, want %q", outcome, LaunchOutcomeAlreadyRunning)
	}
	if starts != 0 {
		t.Fatalf("starts = %d, want 0", starts)
	}
}

func TestClearAccountsRemovesCookiesAndResetsRuntime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cookies := filepath.Join(root, "cookies.json")
	runState := filepath.Join(root, "run.state")
	if err := os.WriteFile(cookies, []byte("cookie"), 0o644); err != nil {
		t.Fatalf("WriteFile(cookies) error = %v", err)
	}
	if err := os.WriteFile(runState, []byte("run"), 0o644); err != nil {
		t.Fatalf("WriteFile(run.state) error = %v", err)
	}

	var stopCalls int
	var startCalls int
	controller := Controller{
		CookiePaths: []string{cookies},
		ResetState: func() error {
			return os.Remove(runState)
		},
		StopAll: func() error {
			stopCalls++
			return nil
		},
		StatusCheck: func() bool { return false },
		StartHelper: func() error {
			startCalls++
			return nil
		},
		InstallRuntime: func() error { return nil },
	}

	if err := controller.ClearAccounts(); err != nil {
		t.Fatalf("ClearAccounts() error = %v", err)
	}
	if _, err := os.Stat(cookies); !os.IsNotExist(err) {
		t.Fatalf("cookies still exists, err=%v", err)
	}
	if _, err := os.Stat(runState); !os.IsNotExist(err) {
		t.Fatalf("runState still exists, err=%v", err)
	}
	if stopCalls != 1 {
		t.Fatalf("stopCalls = %d, want 1", stopCalls)
	}
	if startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1", startCalls)
	}
}

func TestExitStopsManagedRuntime(t *testing.T) {
	t.Parallel()

	var stopCalls int
	controller := Controller{
		StopAll: func() error {
			stopCalls++
			return nil
		},
	}

	if err := controller.Exit(); err != nil {
		t.Fatalf("Exit() error = %v", err)
	}
	if stopCalls != 1 {
		t.Fatalf("stopCalls = %d, want 1", stopCalls)
	}
}
