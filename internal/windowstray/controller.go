package windowstray

import (
	"fmt"
	"os"
)

type LaunchOutcome string

const (
	LaunchOutcomeStarted        LaunchOutcome = "started"
	LaunchOutcomeAlreadyRunning LaunchOutcome = "already_running"
)

type Controller struct {
	StatusCheck    func() bool
	StartHelper    func() error
	InstallRuntime func() error
	CookiePaths    []string
	ResetState     func() error
	StopAll        func() error
}

func (c Controller) EnsureHelperStarted() (LaunchOutcome, error) {
	if c.StatusCheck != nil && c.StatusCheck() {
		return LaunchOutcomeAlreadyRunning, nil
	}
	if c.StartHelper == nil {
		return "", fmt.Errorf("start helper action is required")
	}
	if err := c.StartHelper(); err != nil {
		return "", err
	}
	if c.InstallRuntime != nil {
		if err := c.InstallRuntime(); err != nil {
			return "", err
		}
	}
	return LaunchOutcomeStarted, nil
}

func (c Controller) ClearAccounts() error {
	for _, path := range c.CookiePaths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if c.ResetState != nil {
		if err := c.ResetState(); err != nil {
			return err
		}
	}
	if c.StopAll != nil {
		if err := c.StopAll(); err != nil {
			return err
		}
	}
	_, err := c.EnsureHelperStarted()
	return err
}

func (c Controller) Exit() error {
	if c.StopAll != nil {
		return c.StopAll()
	}
	return nil
}
