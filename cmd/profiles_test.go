package cmd

import (
	"testing"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

func TestApplyLiveExpiryOverridesOnlyTheActiveProfile(t *testing.T) {
	frozen := statusNow.Add(2 * 24 * time.Hour)
	live := statusNow.Add(25 * 24 * time.Hour)
	profiles := []profile.Meta{
		{Name: "work", SessionExpiresAt: frozen},
		{Name: "personal", SessionExpiresAt: frozen},
	}

	applyLiveExpiry(profiles, "work", live)

	if !profiles[0].SessionExpiresAt.Equal(live) {
		t.Fatalf("active profile kept its frozen expiry: %v", profiles[0].SessionExpiresAt)
	}
	if !profiles[1].SessionExpiresAt.Equal(frozen) {
		t.Fatalf("inactive profile was overridden: %v", profiles[1].SessionExpiresAt)
	}
}

func TestApplyLiveExpiryKeepsSnapshotWithoutLiveEvidence(t *testing.T) {
	frozen := statusNow.Add(2 * 24 * time.Hour)
	for _, tc := range []struct {
		name    string
		current string
		live    time.Time
	}{
		{"no active profile", "", statusNow.Add(25 * 24 * time.Hour)},
		{"no live expiry", "work", time.Time{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profiles := []profile.Meta{{Name: "work", SessionExpiresAt: frozen}}
			applyLiveExpiry(profiles, tc.current, tc.live)
			if !profiles[0].SessionExpiresAt.Equal(frozen) {
				t.Fatalf("expiry changed to %v", profiles[0].SessionExpiresAt)
			}
		})
	}
}
