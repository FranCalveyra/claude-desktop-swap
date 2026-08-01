package cmd

import (
	"testing"

	"github.com/FranCalveyra/claude-desktop-swap/internal/account"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

func TestApplyLiveAccountsMarksRejectedProfilesExpired(t *testing.T) {
	profiles := []profile.Meta{
		{Name: "work", ObservedHealth: profile.HealthUsable},
		{Name: "personal", ObservedHealth: profile.HealthUsable},
	}
	infos := map[string]account.Info{
		"work":     {Rejected: true},
		"personal": {Email: "a@b.com", Plan: "Pro"},
	}
	applyLiveAccounts(profiles, infos)

	if profiles[0].ObservedHealth != profile.HealthExpired {
		t.Fatalf("work health = %s, want expired", profiles[0].ObservedHealth)
	}
	if profiles[1].ObservedHealth != profile.HealthUsable {
		t.Fatalf("personal health = %s, want usable", profiles[1].ObservedHealth)
	}
	if profiles[1].Email != "a@b.com" || profiles[1].Plan != "Pro" {
		t.Fatalf("personal not enriched: %+v", profiles[1])
	}
}
