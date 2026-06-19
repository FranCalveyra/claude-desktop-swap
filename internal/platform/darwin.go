//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	appName              = "Claude"
	appDataDir           = "Application Support"
	killPollInterval     = 100 * time.Millisecond
	killMaxPolls         = 50 // 50 × 100ms = 5 second timeout before SIGKILL
	killForceDelay       = 300 * time.Millisecond
	claudeProcessPattern = `/Applications/Claude.app/Contents/MacOS/Claude|Claude Helper`
)

var errProcessAbsent = errors.New("process absent")

type darwinPlatform struct{}

func current() Platform { return &darwinPlatform{} }

func (d *darwinPlatform) AppDataPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", appDataDir, appName), nil
}

func (d *darwinPlatform) IsRunning() (bool, error) {
	err := runProcessCommand("pgrep", "-f", claudeProcessPattern)
	if errors.Is(err, errProcessAbsent) {
		return false, nil
	}
	return err == nil, err
}

func (d *darwinPlatform) KillApp() error {
	return stopClaudeProcesses(runProcessCommand, time.Sleep, killMaxPolls)
}

func (d *darwinPlatform) LaunchApp() error {
	return exec.Command("open", "-a", appName).Run()
}

func runProcessCommand(name string, args ...string) error {
	err := exec.Command(name, args...).Run()
	if _, ok := err.(*exec.ExitError); ok {
		return errProcessAbsent
	}
	return err
}

func stopClaudeProcesses(run func(string, ...string) error, sleep func(time.Duration), polls int) error {
	if err := run("pgrep", "-f", claudeProcessPattern); errors.Is(err, errProcessAbsent) {
		return nil
	} else if err != nil {
		return err
	}
	_ = run("pkill", "-TERM", "-f", claudeProcessPattern)
	for range polls {
		sleep(killPollInterval)
		if err := run("pgrep", "-f", claudeProcessPattern); errors.Is(err, errProcessAbsent) {
			return nil
		}
	}
	_ = run("pkill", "-KILL", "-f", claudeProcessPattern)
	sleep(killForceDelay)
	if err := run("pgrep", "-f", claudeProcessPattern); errors.Is(err, errProcessAbsent) {
		return nil
	}
	return fmt.Errorf("Claude Desktop processes remain after forced termination")
}
