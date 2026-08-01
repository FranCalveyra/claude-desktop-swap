package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

var statusNow = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func TestStatusLineReportsConfirmedMatch(t *testing.T) {
	got := statusLine(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", statusNow.Add(20*24*time.Hour), statusNow)
	if got != "Active profile: work (usable, session in 20d)" {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineDoesNotClaimStaleIdentity(t *testing.T) {
	got := statusLine(fakeMatcher{health: profile.HealthUsable}, "/synthetic", time.Time{}, statusNow)
	if !strings.Contains(got, "unknown") || strings.Contains(got, "work") {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineWarnsInsideRenewalWindow(t *testing.T) {
	got := statusLine(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", statusNow.Add(4*24*time.Hour), statusNow)
	if !strings.Contains(got, "in 4d") || !strings.Contains(got, "expires in 4d") {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineStaysQuietWithoutExpiryEvidence(t *testing.T) {
	got := statusLine(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", time.Time{}, statusNow)
	if strings.Contains(got, "expires") || !strings.Contains(got, "session -") {
		t.Fatalf("status = %q", got)
	}
}

type fakeMatcher struct {
	name   string
	health profile.Health
}

func (m fakeMatcher) MatchLive(string) (string, profile.Health) { return m.name, m.health }
