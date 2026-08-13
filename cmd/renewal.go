package cmd

import (
	"fmt"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

// liveSessionExpiry reads the expiry from live app-data rather than from a
// snapshot. The active profile's snapshot is frozen at checkpoint time while
// the app keeps rotating the live sessionKey, so the snapshot under-reports
// how long the active account actually has.
func liveSessionExpiry(appData string, now time.Time) time.Time {
	return profile.InspectCookies(profile.CookiesPath(appData), now).ExpiresAt
}

func expiryLabel(expiresAt, now time.Time) string {
	switch profile.ClassifyRenewal(expiresAt, now) {
	case profile.RenewalUnknown:
		return "-"
	case profile.RenewalExpired:
		return "expired"
	case profile.RenewalSoon:
		return "in " + until(expiresAt, now) + " ⚠"
	default:
		return "in " + until(expiresAt, now)
	}
}

func until(expiresAt, now time.Time) string {
	remaining := expiresAt.Sub(now)
	if remaining < time.Hour {
		return fmt.Sprintf("%dm", int(remaining.Minutes()))
	}
	if remaining < 24*time.Hour {
		return fmt.Sprintf("%dh", int(remaining.Hours()))
	}
	return fmt.Sprintf("%dd", int(remaining.Hours()/24))
}

// renewalFor reconciles the two signals a profile carries. A session the
// server rejected (401, folded into HealthExpired by enrichLiveAccounts) is
// expired no matter how much life its local cookie claims to have left.
func renewalFor(p profile.Meta, now time.Time) profile.Renewal {
	if p.ObservedHealth == profile.HealthExpired {
		return profile.RenewalExpired
	}
	return profile.ClassifyRenewal(p.SessionExpiresAt, now)
}

func metaExpiryLabel(p profile.Meta, now time.Time) string {
	if renewalFor(p, now) == profile.RenewalExpired {
		return "expired"
	}
	return expiryLabel(p.SessionExpiresAt, now)
}

func renewalNotices(profiles []profile.Meta, now time.Time) []string {
	var notices []string
	for _, p := range profiles {
		switch renewalFor(p, now) {
		case profile.RenewalSoon:
			notices = append(notices, fmt.Sprintf("⚠ %q expires in %s — open Claude on that profile to renew it.", p.Name, until(p.SessionExpiresAt, now)))
		case profile.RenewalExpired:
			notices = append(notices, fmt.Sprintf("⚠ %q has expired — sign in again in Claude Desktop and re-save it.", p.Name))
		}
	}
	return notices
}
