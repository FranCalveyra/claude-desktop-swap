package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

var renewalNow = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

func TestExpiryLabel(t *testing.T) {
	for _, tc := range []struct {
		name      string
		expiresAt time.Time
		want      string
	}{
		{"unknown", time.Time{}, "-"},
		{"expired", renewalNow.Add(-time.Hour), "expired"},
		{"soon", renewalNow.Add(4 * 24 * time.Hour), "in 4d ⚠"},
		{"hours left", renewalNow.Add(5 * time.Hour), "in 5h ⚠"},
		{"healthy", renewalNow.Add(21 * 24 * time.Hour), "in 21d"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := expiryLabel(tc.expiresAt, renewalNow); got != tc.want {
				t.Fatalf("expiryLabel = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenewalNoticesOnlyFlagActionableProfiles(t *testing.T) {
	profiles := []profile.Meta{
		{Name: "work", SessionExpiresAt: renewalNow.Add(21 * 24 * time.Hour)},
		{Name: "personal", SessionExpiresAt: renewalNow.Add(4 * 24 * time.Hour)},
		{Name: "old", SessionExpiresAt: renewalNow.Add(-24 * time.Hour)},
		{Name: "unreadable"},
	}

	notices := renewalNotices(profiles, renewalNow)
	if len(notices) != 2 {
		t.Fatalf("notices = %v, want 2", notices)
	}
	if !strings.Contains(notices[0], `"personal"`) || !strings.Contains(notices[0], "in 4d") {
		t.Fatalf("notice = %q", notices[0])
	}
	if !strings.Contains(notices[1], `"old"`) || !strings.Contains(notices[1], "expired") {
		t.Fatalf("notice = %q", notices[1])
	}
	for _, notice := range notices {
		if strings.Contains(notice, "work") || strings.Contains(notice, "unreadable") {
			t.Fatalf("notice must not flag healthy or unknown profiles: %q", notice)
		}
	}
}

// enrichLiveAccounts folds a server 401 into HealthExpired. The row must not
// then claim the cookie has weeks of life left, and the profile must still
// raise a notice.
func TestServerRejectionOverridesLocalCookieEvidence(t *testing.T) {
	rejected := profile.Meta{
		Name:             "old",
		ObservedHealth:   profile.HealthExpired,
		SessionExpiresAt: renewalNow.Add(20 * 24 * time.Hour),
	}

	if got := metaExpiryLabel(rejected, renewalNow); got != "expired" {
		t.Fatalf("label = %q, want %q", got, "expired")
	}
	notices := renewalNotices([]profile.Meta{rejected}, renewalNow)
	if len(notices) != 1 || !strings.Contains(notices[0], "has expired") {
		t.Fatalf("notices = %v", notices)
	}
}

// An expiring session is still usable — the renewal signal must never leak
// into Health, or the guards in save/use would refuse a valid profile.
func TestExpiringSessionStaysUsable(t *testing.T) {
	if got := profile.ClassifyRenewal(renewalNow.Add(time.Hour), renewalNow); got != profile.RenewalSoon {
		t.Fatalf("renewal = %s, want soon", got)
	}
	for _, h := range []profile.Health{profile.HealthUsable, profile.HealthExpired, profile.HealthMissing, profile.HealthUnknown} {
		if string(h) == string(profile.RenewalSoon) {
			t.Fatalf("Health %q collides with a Renewal value", h)
		}
	}
}
