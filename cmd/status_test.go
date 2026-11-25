package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
	"github.com/muesli/termenv"
)

var statusNow = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func TestStatusLineReportsConfirmedMatch(t *testing.T) {
	got := statusOf(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", statusNow.Add(20*24*time.Hour), statusNow).String()
	if got != "Active profile: work (usable, session in 20d)" {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineDoesNotClaimStaleIdentity(t *testing.T) {
	got := statusOf(fakeMatcher{health: profile.HealthUsable}, "/synthetic", time.Time{}, statusNow).String()
	if !strings.Contains(got, "unknown") || strings.Contains(got, "work") {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineWarnsInsideRenewalWindow(t *testing.T) {
	got := statusOf(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", statusNow.Add(4*24*time.Hour), statusNow).String()
	if !strings.Contains(got, "in 4d") || !strings.Contains(got, "expires in 4d") {
		t.Fatalf("status = %q", got)
	}
}

func TestStatusLineStaysQuietWithoutExpiryEvidence(t *testing.T) {
	got := statusOf(fakeMatcher{name: "work", health: profile.HealthUsable}, "/synthetic", time.Time{}, statusNow).String()
	if strings.Contains(got, "expires") || !strings.Contains(got, "session -") {
		t.Fatalf("status = %q", got)
	}
}

type fakeMatcher struct {
	name   string
	health profile.Health
}

func (m fakeMatcher) MatchLive(string) (string, profile.Health) { return m.name, m.health }

func TestStatusWarningsRenderPerInterface(t *testing.T) {
	r := statusReport{line: "Active profile: work", warnings: []string{"Session has expired"}}
	if got := r.String(); got != "Active profile: work\n⚠ Session has expired" {
		t.Fatalf("cli status = %q", got)
	}
	setTestColorProfile(t, termenv.Ascii)
	if got := styledStatus(r); !strings.Contains(got, "Warning: Session has expired") || strings.Contains(got, "⚠") {
		t.Fatalf("tui status = %q", got)
	}
}
