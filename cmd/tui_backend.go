package cmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/FranCalveyra/claude-desktop-swap/internal/account"
	"github.com/FranCalveyra/claude-desktop-swap/internal/platform"
	"github.com/FranCalveyra/claude-desktop-swap/internal/profile"
)

type defaultTUIBackend struct {
	store      *profile.Store
	platform   platform.Platform
	addAppData string
}

func newDefaultTUIBackend() (*defaultTUIBackend, error) {
	store, err := profile.NewStore()
	if err != nil {
		return nil, err
	}
	return &defaultTUIBackend{store: store, platform: platform.Current()}, nil
}

func (b *defaultTUIBackend) Load() (tuiSnapshot, error) {
	profiles, err := b.store.List()
	if err != nil {
		return tuiSnapshot{}, err
	}
	appData, err := b.platform.AppDataPath()
	if err != nil {
		return tuiSnapshot{}, err
	}
	now := time.Now()
	current, _ := b.store.MatchLive(appData)
	if live := liveSessionExpiry(appData, now); current != "" && !live.IsZero() {
		for i := range profiles {
			if profiles[i].Name == current {
				profiles[i].SessionExpiresAt = live
			}
		}
	}
	enrichLiveAccounts(b.store, profiles)
	return tuiSnapshot{
		profiles: profiles,
		current:  current,
		status:   strings.ReplaceAll(statusLine(b.store, appData, liveSessionExpiry(appData, now), now), "⚠ ", "Warning: "),
		now:      now,
	}, nil
}

func (b *defaultTUIBackend) Activate(name string) error {
	return switchProfileWith(name, b.store, b.platform, io.Discard, account.Fetch)
}

func (b *defaultTUIBackend) Save(name string) error {
	return saveProfileWith(name, b.store, b.platform, io.Discard)
}

func (b *defaultTUIBackend) BeginAdd(name string) error {
	appData, err := beginAddProfileWith(name, b.store, b.platform, io.Discard)
	if err == nil {
		b.addAppData = appData
	}
	return err
}

func (b *defaultTUIBackend) FinishAdd(name string) error {
	if b.addAppData == "" {
		return fmt.Errorf("add session was not prepared")
	}
	err := finishAddProfileWith(name, b.addAppData, b.store, b.platform, io.Discard)
	if err == nil {
		b.addAppData = ""
	}
	return err
}

func (b *defaultTUIBackend) Delete(name string) error {
	return deleteProfileWith(name, b.store, io.Discard)
}
