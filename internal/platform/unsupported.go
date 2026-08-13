//go:build !darwin && !windows

package platform

import (
	"fmt"
	"runtime"
)

type unsupportedPlatform struct{}

func current() Platform { return &unsupportedPlatform{} }

func (u *unsupportedPlatform) AppDataPath() (string, error) {
	return "", fmt.Errorf("platform %q is not yet supported", runtime.GOOS)
}

func (u *unsupportedPlatform) IsRunning() (bool, error) {
	return false, fmt.Errorf("platform %q is not yet supported", runtime.GOOS)
}

func (u *unsupportedPlatform) KillApp() error {
	return fmt.Errorf("platform %q is not yet supported", runtime.GOOS)
}

func (u *unsupportedPlatform) LaunchApp() error {
	return fmt.Errorf("platform %q is not yet supported", runtime.GOOS)
}
