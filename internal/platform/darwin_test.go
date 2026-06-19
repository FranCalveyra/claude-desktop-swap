//go:build darwin

package platform

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStopClaudeProcessesTerminatesAppAndHelpers(t *testing.T) {
	var calls [][]string
	runningChecks := 0
	run := func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		if name == "pgrep" {
			runningChecks++
			if runningChecks > 1 {
				return errProcessAbsent
			}
		}
		return nil
	}
	if err := stopClaudeProcesses(run, func(time.Duration) {}, 2); err != nil {
		t.Fatal(err)
	}
	wantFirst := []string{"pkill", "-TERM", "-f", claudeProcessPattern}
	if !reflect.DeepEqual(calls[1], wantFirst) {
		t.Fatalf("termination = %v, want %v", calls[1], wantFirst)
	}
}

func TestStopClaudeProcessesFailsWhenAnyProcessRemains(t *testing.T) {
	run := func(name string, args ...string) error {
		if name == "pgrep" {
			return nil
		}
		return nil
	}
	err := stopClaudeProcesses(run, func(time.Duration) {}, 2)
	if err == nil || errors.Is(err, errProcessAbsent) {
		t.Fatalf("error = %v, want quiescence failure", err)
	}
}
