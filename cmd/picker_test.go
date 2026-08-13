package cmd

import (
	"strings"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPickerShowsHealthWithoutSecretData(t *testing.T) {
	setTestColorProfile(t, termenv.Ascii)
	m := newPickerModel([]profile.Meta{{Name: "work", ObservedHealth: profile.HealthExpired}}, "work")
	view := m.View()
	if !strings.Contains(view, "expired") {
		t.Fatalf("view = %q", view)
	}
	if strings.Contains(view, "sessionKey") || strings.Contains(view, "secret") {
		t.Fatalf("view exposed session data: %q", view)
	}
}

func TestPickerUsesCyanFocusAndSemanticHealthColors(t *testing.T) {
	setTestColorProfile(t, termenv.ANSI)
	m := newPickerModel([]profile.Meta{
		{Name: "usable", ObservedHealth: profile.HealthUsable},
		{Name: "expired", ObservedHealth: profile.HealthExpired},
		{Name: "unknown", ObservedHealth: profile.HealthUnknown},
	}, "")

	view := m.View()
	for _, code := range []string{"96m", "36m", "92m", "91m", "93m"} {
		if !strings.Contains(view, code) {
			t.Fatalf("view missing ANSI color %q: %q", code, view)
		}
	}
}

func TestPickerBlocksUnusableSelection(t *testing.T) {
	for _, health := range []profile.Health{profile.HealthExpired, profile.HealthMissing, profile.HealthUnknown} {
		m := newPickerModel([]profile.Meta{{Name: "work", ObservedHealth: health}}, "")
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		got := updated.(pickerModel)
		if got.chosen != "" {
			t.Fatalf("health %s selected %q", health, got.chosen)
		}
	}
}

func TestPickerAllowsUsableSelection(t *testing.T) {
	m := newPickerModel([]profile.Meta{{Name: "work", ObservedHealth: profile.HealthUsable}}, "")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := updated.(pickerModel).chosen; got != "work" {
		t.Fatalf("chosen = %q", got)
	}
}

func setTestColorProfile(t *testing.T, profile termenv.Profile) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(profile)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}
