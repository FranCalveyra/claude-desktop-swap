package cmd

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

func TestCheckpointTrackedSessionUsesAtomicCheckpoint(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events, current: "outgoing", health: profile.HealthUsable}
	if err := checkpointTrackedSession(store, "/synthetic"); err != nil {
		t.Fatal(err)
	}
	if !containsEvent(events, "checkpoint:outgoing") {
		t.Fatalf("events = %v", events)
	}
}

func TestCheckpointTrackedSessionRequiresIdentity(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events}
	if err := checkpointTrackedSession(store, "/synthetic"); err == nil {
		t.Fatal("untracked session should be refused")
	}
}

func TestBeginAddProfilePreparesFreshSession(t *testing.T) {
	events := []string{}
	store := &fakeAddStore{events: &events}
	p := &fakePlatform{events: &events, appData: t.TempDir(), running: true}

	appData, err := beginAddProfileWith("work", store, p, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if appData != p.appData {
		t.Fatalf("appData = %q", appData)
	}
	want := []string{"app-data", "exists:work", "stop", "wipe", "launch"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestFinishAddProfileStopsCheckpointsAndRestores(t *testing.T) {
	events := []string{}
	store := &fakeAddStore{events: &events}
	p := &fakePlatform{events: &events, appData: t.TempDir()}

	if err := finishAddProfileWith("work", p.appData, store, p, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"stop", "checkpoint:work", "restore:work"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

type fakeAddStore struct {
	events *[]string
}

func (s *fakeAddStore) Exists(name string) bool {
	*s.events = append(*s.events, "exists:"+name)
	return false
}

func (s *fakeAddStore) Current() (string, error) { return "", nil }

func (s *fakeAddStore) Checkpoint(name, path string) error {
	*s.events = append(*s.events, "checkpoint:"+name)
	return nil
}

func (s *fakeAddStore) Restore(name, path string) error {
	*s.events = append(*s.events, "restore:"+name)
	return nil
}

func (s *fakeAddStore) Wipe(string) error {
	*s.events = append(*s.events, "wipe")
	return nil
}
