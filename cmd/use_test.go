package cmd

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

func TestSwitchProfileOrdersStopCheckpointRestoreAndLaunch(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events, exists: true, current: "outgoing", health: profile.HealthUsable}
	p := &fakePlatform{events: &events, appData: t.TempDir()}
	if err := switchProfileWith("incoming", store, p, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"app-data", "inspect:incoming", "stop", "current", "checkpoint:outgoing", "restore:incoming", "launch"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
	if store.current != "incoming" {
		t.Fatalf("current = %q", store.current)
	}
}

func TestSwitchProfileStopsAfterCheckpointOrRestoreFailure(t *testing.T) {
	tests := []struct {
		name                      string
		checkpointErr, restoreErr error
		forbidden                 string
	}{
		{"checkpoint failure", errors.New("checkpoint failed"), nil, "restore:incoming"},
		{"restore interruption", nil, errors.New("restore failed"), "launch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := []string{}
			store := &fakeSwitchStore{events: &events, exists: true, current: "outgoing", health: profile.HealthUsable, checkpointErr: tt.checkpointErr, restoreErr: tt.restoreErr}
			p := &fakePlatform{events: &events, appData: t.TempDir()}
			if err := switchProfileWith("incoming", store, p, &bytes.Buffer{}); err == nil {
				t.Fatal("switch should fail")
			}
			if strings.Contains(strings.Join(events, ","), tt.forbidden) {
				t.Fatalf("forbidden event %q in %v", tt.forbidden, events)
			}
			if store.current != "outgoing" {
				t.Fatalf("tracking advanced to %q", store.current)
			}
		})
	}
}

func TestSwitchProfileRefusesUnusableTargetBeforeStopping(t *testing.T) {
	for _, health := range []profile.Health{profile.HealthExpired, profile.HealthMissing, profile.HealthUnknown} {
		t.Run(string(health), func(t *testing.T) {
			events := []string{}
			store := &fakeSwitchStore{events: &events, exists: true, health: health}
			p := &fakePlatform{events: &events, appData: t.TempDir()}
			if err := switchProfileWith("incoming", store, p, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "reauthentication") {
				t.Fatalf("error = %v", err)
			}
			if containsEvent(events, "stop") {
				t.Fatalf("stopped app for unusable target: %v", events)
			}
		})
	}
}

func TestSwitchProfileLaunchFailureKeepsCommittedIncomingState(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events, exists: true, current: "outgoing", health: profile.HealthUsable}
	p := &fakePlatform{events: &events, appData: t.TempDir(), launchErr: errors.New("launch failed")}
	err := switchProfileWith("incoming", store, p, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "launch manually") {
		t.Fatalf("error = %v", err)
	}
	if store.current != "incoming" {
		t.Fatalf("committed current = %q", store.current)
	}
}

type fakeSwitchStore struct {
	events        *([]string)
	exists        bool
	current       string
	health        profile.Health
	checkpointErr error
	restoreErr    error
}

func (s *fakeSwitchStore) Exists(string) bool { return s.exists }
func (s *fakeSwitchStore) Inspect(name string) profile.Inspection {
	*s.events = append(*s.events, "inspect:"+name)
	return profile.Inspection{Health: s.health}
}
func (s *fakeSwitchStore) Current() (string, error) {
	*s.events = append(*s.events, "current")
	return s.current, nil
}
func (s *fakeSwitchStore) Checkpoint(name, path string) error {
	*s.events = append(*s.events, "checkpoint:"+name)
	return s.checkpointErr
}
func (s *fakeSwitchStore) Restore(name, path string) error {
	*s.events = append(*s.events, "restore:"+name)
	if s.restoreErr == nil {
		s.current = name
	}
	return s.restoreErr
}

type fakePlatform struct {
	events    *([]string)
	appData   string
	launchErr error
}

func (p *fakePlatform) AppDataPath() (string, error) {
	*p.events = append(*p.events, "app-data")
	return p.appData, nil
}
func (p *fakePlatform) IsRunning() (bool, error) { return false, nil }
func (p *fakePlatform) KillApp() error           { *p.events = append(*p.events, "stop"); return nil }
func (p *fakePlatform) LaunchApp() error         { *p.events = append(*p.events, "launch"); return p.launchErr }

func containsEvent(events []string, event string) bool {
	for _, got := range events {
		if got == event {
			return true
		}
	}
	return false
}
