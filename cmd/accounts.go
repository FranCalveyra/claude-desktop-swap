package cmd

import (
	"github.com/FranCalveyra/claude-desktop-swap/internal/account"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

// enrichLiveAccounts overlays live Claude.ai account info (email, plan) onto the
// given profiles, keeping any cached values when a live lookup comes back empty
// (offline, expired session, or unsupported platform). Profiles the server
// explicitly rejects (401) are marked expired so list/picker won't offer them.
func enrichLiveAccounts(store *profile.Store, profiles []profile.Meta) {
	if len(profiles) == 0 {
		return
	}
	paths := make(map[string]string, len(profiles))
	for _, p := range profiles {
		paths[p.Name] = store.ProfileCookiesPath(p.Name)
	}
	applyLiveAccounts(profiles, account.FetchMany(paths))
}

// applyLiveAccounts merges fetched account info into profiles in place.
func applyLiveAccounts(profiles []profile.Meta, infos map[string]account.Info) {
	for i := range profiles {
		info := infos[profiles[i].Name]
		if info.Email != "" {
			profiles[i].Email = info.Email
		}
		if info.Plan != "" {
			profiles[i].Plan = info.Plan
		}
		if info.Rejected {
			profiles[i].ObservedHealth = profile.HealthExpired
		}
	}
}
