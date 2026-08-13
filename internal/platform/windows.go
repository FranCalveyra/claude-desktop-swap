//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	appName          = "Claude"
	processImageName = "claude.exe"
	killPollInterval = 100 * time.Millisecond
	killMaxPolls     = 50 // 50 × 100ms = 5 second timeout before a forced kill
	killForceDelay   = 300 * time.Millisecond
	launchMaxPolls   = 50

	// packageFamilyName identifies the Store (MSIX) build. The suffix is
	// derived from Anthropic's publisher identity, so it is stable across
	// app versions.
	packageFamilyName = "Claude_pzs8sxrjxfjjc"
	// appsFolderID addresses the Store build as <PackageFamilyName>!<AppId>.
	// That build installs under a versioned WindowsApps directory which is
	// not listable without elevation, so there is no exe path to resolve and
	// shell activation is the only way in.
	appsFolderID = `shell:AppsFolder\` + packageFamilyName + `!Claude`
	// standaloneExe is where the non-Store installer puts Claude Desktop,
	// relative to %LOCALAPPDATA%.
	standaloneExe = `AnthropicClaude\claude.exe`
)

type windowsPlatform struct{}

func current() Platform { return &windowsPlatform{} }

// AppDataPath resolves Claude's app-data. The Store build is MSIX-packaged:
// %APPDATA%\Claude is only the container's projection of the package's
// private store, and it exists solely while the app is running — save and use
// both work with the app stopped, so they must address the backing store
// directly or find nothing there. The standalone installer is unpackaged and
// keeps a real %APPDATA%\Claude, so that remains the fallback.
func (w *windowsPlatform) AppDataPath() (string, error) {
	if local, err := os.UserCacheDir(); err == nil {
		packaged := filepath.Join(local, "Packages", packageFamilyName, "LocalCache", "Roaming", appName)
		if _, err := os.Stat(packaged); err == nil {
			return packaged, nil
		}
	}
	roaming, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(roaming, appName), nil
}

func (w *windowsPlatform) IsRunning() (bool, error) {
	pids, err := w.processes()
	return len(pids) > 0, err
}

func (w *windowsPlatform) KillApp() error {
	return stopClaudeProcesses(w.processes, taskkill, time.Sleep, killMaxPolls)
}

func (w *windowsPlatform) LaunchApp() error {
	if err := w.startApp(); err != nil {
		return err
	}
	return awaitRunning(w.processes, time.Sleep, launchMaxPolls)
}

func (w *windowsPlatform) startApp() error {
	if local, err := os.UserCacheDir(); err == nil {
		exe := filepath.Join(local, standaloneExe)
		if _, err := os.Stat(exe); err == nil {
			return exec.Command(exe).Start()
		}
	}
	// explorer.exe exits 1 whether or not the activation succeeded, so its
	// status carries no information; awaitRunning is what confirms the launch.
	_ = exec.Command("explorer.exe", appsFolderID).Run()
	return nil
}

func (w *windowsPlatform) processes() ([]uint32, error) {
	appData, err := w.AppDataPath()
	if err != nil {
		return nil, err
	}
	return claudeProcesses(appData)
}

func stopClaudeProcesses(list func() ([]uint32, error), kill func(uint32, bool) error, sleep func(time.Duration), polls int) error {
	pids, err := list()
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	for _, pid := range pids {
		_ = kill(pid, false)
	}
	for range polls {
		sleep(killPollInterval)
		if remaining, err := list(); err == nil && len(remaining) == 0 {
			return nil
		}
	}
	remaining, err := list()
	if err != nil {
		return err
	}
	for _, pid := range remaining {
		_ = kill(pid, true)
	}
	sleep(killForceDelay)
	if remaining, err := list(); err == nil && len(remaining) == 0 {
		return nil
	}
	return fmt.Errorf("processes for Claude Desktop remain after forced termination")
}

func awaitRunning(list func() ([]uint32, error), sleep func(time.Duration), polls int) error {
	for range polls {
		if pids, err := list(); err == nil && len(pids) > 0 {
			return nil
		}
		sleep(killPollInterval)
	}
	return fmt.Errorf("no Claude Desktop process appeared after launch")
}

// taskkill closes a process the way a user would — /PID alone posts WM_CLOSE
// to its top-level windows — or, forced, terminates its whole tree.
func taskkill(pid uint32, force bool) error {
	args := []string{"/PID", strconv.FormatUint(uint64(pid), 10)}
	if force {
		args = append(args, "/F", "/T")
	}
	return exec.Command("taskkill", args...).Run()
}

// claudeProcesses lists the Claude Desktop process ids. Matching on the image
// name alone is not enough: Claude Code installs its own claude.exe inside the
// app-data tree, so an unfiltered match would kill the user's CLI sessions.
// Processes whose image path cannot be read are skipped rather than assumed
// to be Claude Desktop's.
func claudeProcesses(appDataPath string) ([]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var pids []uint32
	for err := windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), processImageName) {
			continue
		}
		exe, err := processImagePath(entry.ProcessID)
		if err != nil || isBundledCLI(exe, appDataPath) {
			continue
		}
		pids = append(pids, entry.ProcessID)
	}
	return pids, nil
}

// isBundledCLI reports whether an executable lives inside the app-data tree,
// which is where Claude Code installs its own claude.exe.
func isBundledCLI(exePath, appDataPath string) bool {
	prefix := strings.ToLower(appDataPath) + string(filepath.Separator)
	return strings.HasPrefix(strings.ToLower(exePath), prefix)
}

func processImagePath(pid uint32) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return "", err
	}
	return windows.UTF16ToString(buf[:size]), nil
}
