//go:build windows

package platform

import (
	"testing"
	"time"
)

func TestStopClaudeProcessesTerminatesAppAndHelpers(t *testing.T) {
	alive := []uint32{100, 101, 102}
	var killed []uint32
	var forced bool
	list := func() ([]uint32, error) { return alive, nil }
	kill := func(pid uint32, force bool) error {
		killed = append(killed, pid)
		forced = forced || force
		alive = nil
		return nil
	}
	if err := stopClaudeProcesses(list, kill, func(time.Duration) {}, 2); err != nil {
		t.Fatal(err)
	}
	if len(killed) != 3 {
		t.Fatalf("killed = %v, want all three processes", killed)
	}
	if forced {
		t.Fatal("forced termination used on a process that closed gracefully")
	}
}

func TestStopClaudeProcessesFailsWhenAnyProcessRemains(t *testing.T) {
	list := func() ([]uint32, error) { return []uint32{100}, nil }
	kill := func(uint32, bool) error { return nil }
	if err := stopClaudeProcesses(list, kill, func(time.Duration) {}, 2); err == nil {
		t.Fatal("error = nil, want quiescence failure")
	}
}

func TestIsBundledCLIExcludesClaudeCodeFromTheKillSet(t *testing.T) {
	appData := `C:\Users\franc\AppData\Roaming\Claude`
	cases := []struct {
		exe  string
		want bool
	}{
		{`C:\Users\franc\AppData\Roaming\Claude\claude-code\2.1.219\claude.exe`, true},
		{`c:\users\franc\appdata\roaming\claude\claude-code\2.1.205\claude.exe`, true},
		{`C:\Program Files\WindowsApps\Claude_1.24012.9.0_x64__pzs8sxrjxfjjc\app\claude.exe`, false},
		{`C:\Users\franc\AppData\Local\AnthropicClaude\claude.exe`, false},
	}
	for _, tc := range cases {
		if got := isBundledCLI(tc.exe, appData); got != tc.want {
			t.Errorf("isBundledCLI(%q) = %v, want %v", tc.exe, got, tc.want)
		}
	}
}

func TestAwaitRunningFailsWhenNothingStarts(t *testing.T) {
	list := func() ([]uint32, error) { return nil, nil }
	if err := awaitRunning(list, func(time.Duration) {}, 2); err == nil {
		t.Fatal("error = nil, want launch failure")
	}
}
