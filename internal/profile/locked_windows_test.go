//go:build windows

package profile

import (
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// lockExclusive holds path open denying every sharing mode, the way Claude
// Desktop holds its live cookie database while it runs.
func lockExclusive(t *testing.T, path string) {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, 0, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = windows.CloseHandle(handle) })
}

func TestInspectCookiesReportsALockedDatabaseAsLocked(t *testing.T) {
	live := syntheticAppData(t, "work")
	path := CookiesPath(live)
	lockExclusive(t, path)

	got := InspectCookies(path, time.Now())
	if !got.Locked {
		t.Fatalf("Locked = false, want true (health %s: %s)", got.Health, got.Reason)
	}
	if got.Health != HealthUnknown {
		t.Fatalf("Health = %s, want unknown — a lock says nothing about the contents", got.Health)
	}
}

// A running Claude Desktop must not make status report "unknown": the lock is
// evidence of a live session, and identity comes from config.json regardless.
func TestMatchLiveIdentifiesAccountWhileClaudeHoldsCookies(t *testing.T) {
	store := newTestStore(t)
	saved := syntheticAppData(t, "work")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}

	live := syntheticAppData(t, "work")
	lockExclusive(t, CookiesPath(live))

	if name, health := store.MatchLive(live); name != "work" || health != HealthUsable {
		t.Fatalf("match = %q/%s, want work/usable", name, health)
	}
}
