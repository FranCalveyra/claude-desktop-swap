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
	addPrior   string
}

func newDefaultTUIBackend() (*defaultTUIBackend, error) {
	store, err := profile.NewStore()
	if err != nil {
		return nil, err
	}
	return &defaultTUIBackend{store: store, platform: platform.Current()}, nil
}

func (b *defaultTUIBackend) Load() (tuiSnapshot, error) {
	if _, err := b.platform.AppDataPath(); err != nil {
		return tuiSnapshot{}, err
	}
	now := time.Now()
	set, err := loadProfiles(b.store, b.platform, now)
	if err != nil {
		return tuiSnapshot{}, err
	}
	return tuiSnapshot{
		profiles: set.profiles,
		current:  set.current,
		status:   strings.ReplaceAll(statusLine(b.store, set.appData, set.liveExpiry, now), "⚠ ", "Warning: "),
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
	prior, err := priorTrackedSession(b.store, b.platform)
	if err != nil {
		return err
	}
	appData, err := beginAddProfileWith(name, b.store, b.platform, io.Discard)
	if err == nil {
		b.addAppData = appData
		b.addPrior = prior
	}
	return err
}

func (b *defaultTUIBackend) CancelAdd() error {
	if b.addAppData == "" {
		return fmt.Errorf("add session was not prepared")
	}
	err := cancelAddProfileWith(b.addPrior, b.addAppData, b.store, b.platform)
	if err == nil {
		b.addAppData, b.addPrior = "", ""
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
