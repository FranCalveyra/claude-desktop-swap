package cmd

import (
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

type profileSet struct {
	profiles   []profile.Meta
	current    string
	appData    string
	liveExpiry time.Time
}

// loadProfiles is what every profile view shows: saved profiles, the one the
// live app-data belongs to, and live account info. appData is empty when the
// platform has no app-data path, which views treat as "nothing is active".
func loadProfiles(store *profile.Store, p platform.Platform, now time.Time) (profileSet, error) {
	profiles, err := store.List()
	if err != nil {
		return profileSet{}, err
	}
	set := profileSet{profiles: profiles}
	if appData, err := p.AppDataPath(); err == nil {
		set.appData = appData
		set.current, _ = store.MatchLive(appData)
		set.liveExpiry = liveSessionExpiry(appData, now)
		applyLiveExpiry(set.profiles, set.current, set.liveExpiry)
	}
	enrichLiveAccounts(store, set.profiles)
	return set, nil
}

// applyLiveExpiry overrides the active profile's snapshot expiry, which froze
// at checkpoint time while the app kept rotating the live sessionKey.
func applyLiveExpiry(profiles []profile.Meta, current string, live time.Time) {
	if current == "" || live.IsZero() {
		return
	}
	for i := range profiles {
		if profiles[i].Name == current {
			profiles[i].SessionExpiresAt = live
		}
	}
}
