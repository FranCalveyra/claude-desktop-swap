package cmd

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSaveProfileStopsCheckpointsAndRelaunchesWhenRunning(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events}
	p := &fakePlatform{events: &events, appData: t.TempDir(), running: true}
	if err := saveProfileWith("work", store, p, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"app-data", "stop", "checkpoint:work", "inspect:work", "launch"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestSaveProfileCheckpointsWithoutTouchingAppWhenStopped(t *testing.T) {
	events := []string{}
	store := &fakeSwitchStore{events: &events}
	p := &fakePlatform{events: &events, appData: t.TempDir(), running: false}
	if err := saveProfileWith("work", store, p, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	want := []string{"app-data", "checkpoint:work", "inspect:work"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}
