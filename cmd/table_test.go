package cmd

import (
	"strings"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/muesli/termenv"
)

func TestProfileTableHeaderAlignsWithRows(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	profiles := []profile.Meta{{Name: "work", Email: "person@example.com", ObservedHealth: profile.HealthUsable}}

	views := map[string]string{
		"picker":    newPickerModel(profiles, "").View(),
		"dashboard": tuiModelWithSnapshot(&fakeTUIBackend{}, tuiSnapshot{profiles: profiles}).View(),
	}
	for name, view := range views {
		t.Run(name, func(t *testing.T) {
			header, row := -1, -1
			for _, line := range strings.Split(view, "\n") {
				if i := strings.Index(line, "ACCOUNT"); i >= 0 {
					header = i
				}
				if i := strings.Index(line, "person@example.com"); i >= 0 {
					row = i
				}
			}
			if header < 0 || header != row {
				t.Fatalf("ACCOUNT header at column %d, account value at %d:\n%s", header, row, view)
			}
		})
	}
}
