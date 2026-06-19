package cmd

import (
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
