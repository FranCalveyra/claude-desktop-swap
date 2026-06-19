package cmd

import (
	"strings"
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

func TestStatusLineReportsConfirmedMatch(t *testing.T) {
	got := statusLine(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic")
	if got != "Active profile: work (usable)" {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineDoesNotClaimStaleIdentity(t *testing.T) {
	got := statusLine(fakeMatcher{health: profile.HealthUsable}, "/synthetic")
	if !strings.Contains(got, "unknown") || strings.Contains(got, "work") {
		t.Fatalf("status = %q", got)
	}
}

type fakeMatcher struct {
	name   string
	health profile.Health
}

func (m fakeMatcher) MatchLive(string) (string, profile.Health) { return m.name, m.health }
