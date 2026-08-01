package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	storeDirName       = ".claude-swap"
	profilesDirName    = "profiles"
	currentFileName    = "current"
	cookiesFile        = "Cookies"
	cookiesJournalFile = "Cookies-journal"
	cookiesWALFile     = "Cookies-wal"
	cookiesSHMFile     = "Cookies-shm"
	configFile         = "config.json"
	rollbackDirName    = ".rollback"
	metaFile           = "meta.json"
	formatVersion      = 3

	dirPerm  os.FileMode = 0700
	filePerm os.FileMode = 0600
)

// cacheDenylist covers app-data entries that are pure cache/runtime state:
// large, regenerable by Chromium/Electron on next launch, and never worth
// carrying between accounts.
var cacheDenylist = map[string]bool{
	"Cache":                     true,
	"Code Cache":                true,
	"GPUCache":                  true,
	"DawnGraphiteCache":         true,
	"DawnWebGPUCache":           true,
	"blob_storage":              true,
	"Crashpad":                  true,
	"sentry":                    true,
	"claude-code":               true,
	"claude-code-vm":            true,
	"ca-bundle.pem":             true,
	"extensions-blocklist.json": true,
}

// workDenylist covers entries that describe the machine or in-flight work
// rather than the signed-in account: they must survive an account switch
// untouched and never get captured into a profile.
var workDenylist = map[string]bool{
	"ant-did":                     true,
	"claude_desktop_config.json":  true,
	"claude-code-sessions":        true,
	"local-agent-mode-sessions":   true,
	"vm_bundles":                  true,
	"git-worktrees.json":          true,
	"window-state.json":           true,
	"plan-usage-history.json":     true,
	"buddy-tokens.json":           true,
	"cowork-enabled-cli-ops.json": true,
}

// skip reports whether a top-level app-data entry is cache/runtime state or
// machine/work state, and therefore must never be captured into or mirrored
// out of a profile. Everything else belongs to the account by default, so
// new state Anthropic adds later is captured automatically.
func skip(name string) bool {
	return cacheDenylist[name] || workDenylist[name]
}

type Meta struct {
	Name           string    `json:"name"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsed       time.Time `json:"last_used,omitempty"`
	Email          string    `json:"email,omitempty"`
	Plan           string    `json:"plan,omitempty"`
	FormatVersion  int       `json:"format_version,omitempty"`
	SavedAt        time.Time `json:"saved_at,omitempty"`
	ObservedHealth Health    `json:"observed_health,omitempty"`
	CookieDigest   string    `json:"cookie_digest,omitempty"`
	// AccountUUID is config.json's lastKnownAccountUuid at checkpoint time —
	// the account identity used by MatchLive. CookieDigest is not identity:
	// the sessionKey cookie is a sliding 28-day token the app rotates on its
	// own, so a live digest routinely diverges from what was last saved.
	AccountUUID string `json:"account_uuid,omitempty"`
}

type Store struct {
	baseDir string
	now     func() time.Time
}

func NewStore() (*Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return newStore(filepath.Join(home, storeDirName))
}

func newStore(base string) (*Store, error) {
	profiles := filepath.Join(base, profilesDirName)
	if err := os.MkdirAll(profiles, dirPerm); err != nil {
		return nil, err
	}
	if err := os.Chmod(base, dirPerm); err != nil {
		return nil, err
	}
	if err := os.Chmod(profiles, dirPerm); err != nil {
		return nil, err
	}
	s := &Store{baseDir: base, now: time.Now}
	if err := s.recoverProfiles(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Exists(name string) bool {
	info, err := os.Stat(s.profileDir(name))
	return err == nil && info.IsDir()
}

func (s *Store) Save(name, appDataPath string) error {
	return s.Checkpoint(name, appDataPath)
}

func (s *Store) Checkpoint(name, appDataPath string) error {
	live := filepath.Join(appDataPath, cookiesFile)
	if inspection := InspectCookies(live, s.now()); inspection.Health != HealthUsable {
		return fmt.Errorf("refuse checkpoint of %s session: %s", inspection.Health, inspection.Reason)
	}
	if err := CheckpointCookies(live); err != nil {
		return fmt.Errorf("checkpoint Cookies WAL: %w", err)
	}
	if inspection := InspectCookies(live, s.now()); inspection.Health != HealthUsable {
		return fmt.Errorf("refuse checkpoint of %s session: %s", inspection.Health, inspection.Reason)
	}

	stage, err := os.MkdirTemp(s.profilesPath(), "."+name+".stage-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, dirPerm); err != nil {
		return err
	}
	if err := captureAppData(appDataPath, stage); err != nil {
		return err
	}

	stagedCookies := filepath.Join(stage, cookiesFile)
	digest, err := cookieDigest(stagedCookies)
	if err != nil {
		return err
	}
	// Best-effort: an unreadable or missing config.json degrades identity
	// tracking (MatchLive) but must not block saving the profile itself.
	accountUUID, _ := readAccountUUID(stage)
	meta := Meta{Name: name, CreatedAt: s.now(), FormatVersion: formatVersion, SavedAt: s.now(), ObservedHealth: HealthUsable, CookieDigest: digest, AccountUUID: accountUUID}
	if existing, err := s.loadMeta(name); err == nil {
		meta.CreatedAt = existing.CreatedAt
		meta.LastUsed = existing.LastUsed
		meta.Email = existing.Email
		meta.Plan = existing.Plan
	}
	if err := writeJSONAtomic(filepath.Join(stage, metaFile), meta); err != nil {
		return err
	}
	if inspection := InspectCookies(stagedCookies, s.now()); inspection.Health != HealthUsable {
		return fmt.Errorf("staged profile is %s: %s", inspection.Health, inspection.Reason)
	}
	if err := syncTree(stage); err != nil {
		return err
	}
	return s.commitProfile(name, stage)
}

// captureAppData copies every top-level app-data entry that isn't
// denylisted into stage, recursively preserving the tree underneath.
func captureAppData(appDataPath, stage string) error {
	entries, err := os.ReadDir(appDataPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == rollbackDirName || skip(name) {
			continue
		}
		src := filepath.Join(appDataPath, name)
		dst := filepath.Join(stage, name)
		if entry.IsDir() {
			if err := copyDir(src, dst); err != nil {
				return fmt.Errorf("capture %s: %w", name, err)
			}
		} else if entry.Type().IsRegular() {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("capture %s: %w", name, err)
			}
		}
	}
	return nil
}

func (s *Store) Inspect(name string) Inspection {
	if !s.Exists(name) {
		return Inspection{Health: HealthMissing, Reason: "profile is missing"}
	}
	if err := validateSecureTree(s.profileDir(name)); err != nil {
		return Inspection{Health: HealthUnknown, Reason: err.Error()}
	}
	cookies := filepath.Join(s.profileDir(name), cookiesFile)
	inspection := InspectCookies(cookies, s.now())
	if inspection.Health != HealthUsable {
		return inspection
	}
	meta, err := s.loadMeta(name)
	if err == nil {
		if meta.FormatVersion < formatVersion {
			return Inspection{Health: HealthUnknown, Reason: "profile is in an outdated format — run `save <name>` while signed into that account to re-save it"}
		}
		if meta.CookieDigest != "" {
			digest, err := cookieDigest(cookies)
			if err != nil || digest != meta.CookieDigest {
				return Inspection{Health: HealthUnknown, Reason: "profile integrity digest does not match"}
			}
		}
	}
	return inspection
}

// Restore mirrors the profile onto live app-data: every live top-level entry
// that isn't denylisted is moved into a same-directory rollback backup, the
// profile is copied in its place, and the backup is discarded only once the
// whole copy has committed without error. Any failure along the way restores
// the exact pre-Restore tree from the backup.
func (s *Store) Restore(name, appDataPath string) error {
	inspection := s.Inspect(name)
	if inspection.Health != HealthUsable {
		return fmt.Errorf("profile %q is %s: %s", name, inspection.Health, inspection.Reason)
	}
	if err := os.MkdirAll(appDataPath, dirPerm); err != nil {
		return err
	}
	if err := recoverRollback(appDataPath); err != nil {
		return fmt.Errorf("recover interrupted restore: %w", err)
	}

	rollbackDir := filepath.Join(appDataPath, rollbackDirName)
	if err := os.MkdirAll(rollbackDir, dirPerm); err != nil {
		return err
	}

	liveEntries, err := os.ReadDir(appDataPath)
	if err != nil {
		return err
	}
	var moved []string
	var copied []string
	rollback := func() { rollbackRestore(appDataPath, rollbackDir, copied, moved) }

	for _, entry := range liveEntries {
		name := entry.Name()
		if name == rollbackDirName || skip(name) {
			continue
		}
		if err := os.Rename(filepath.Join(appDataPath, name), filepath.Join(rollbackDir, name)); err != nil {
			rollback()
			return fmt.Errorf("retain live %s: %w", name, err)
		}
		moved = append(moved, name)
	}

	profileDir := s.profileDir(name)
	profileEntries, err := os.ReadDir(profileDir)
	if err != nil {
		rollback()
		return err
	}
	for _, entry := range profileEntries {
		entryName := entry.Name()
		if entryName == metaFile {
			continue
		}
		src := filepath.Join(profileDir, entryName)
		dst := filepath.Join(appDataPath, entryName)
		var copyErr error
		if entry.IsDir() {
			copyErr = copyDir(src, dst)
		} else if entry.Type().IsRegular() {
			copyErr = copyFile(src, dst)
		}
		if copyErr != nil {
			rollback()
			return fmt.Errorf("restore %s: %w", entryName, copyErr)
		}
		copied = append(copied, entryName)
	}

	live := filepath.Join(appDataPath, cookiesFile)
	if got := InspectCookies(live, s.now()); got.Health != HealthUsable {
		rollback()
		return fmt.Errorf("restored live cookies are %s: %s", got.Health, got.Reason)
	}
	if err := os.Chmod(live, filePerm); err != nil {
		rollback()
		return err
	}
	if err := StripVolatileCookies(live); err != nil {
		rollback()
		return fmt.Errorf("strip volatile cookies: %w", err)
	}

	previousMeta, metaErr := s.loadMeta(name)
	if err := s.setLastUsed(name); err != nil {
		rollback()
		return err
	}
	if err := s.SetCurrent(name); err != nil {
		if metaErr == nil {
			_ = s.saveMeta(name, previousMeta)
		}
		rollback()
		return err
	}

	return os.RemoveAll(rollbackDir)
}

// recoverRollback finishes an interrupted Restore found on the next run: for
// each entry still sitting in a leftover rollback backup, the live slot is
// either empty (the interrupted copy never reached it — rename the backup
// back) or already holds the entry the interrupted copy placed there (which
// is what the very next Restore attempt will overwrite anyway — discard the
// now-redundant backup copy). It never removes anything actually live.
func recoverRollback(appDataPath string) error {
	rollbackDir := filepath.Join(appDataPath, rollbackDirName)
	info, err := os.Stat(rollbackDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(rollbackDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		dst := filepath.Join(appDataPath, name)
		src := filepath.Join(rollbackDir, name)
		if _, err := os.Stat(dst); errors.Is(err, os.ErrNotExist) {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("recover %s: %w", name, err)
			}
		} else if err != nil {
			return err
		} else {
			if err := os.RemoveAll(src); err != nil {
				return fmt.Errorf("discard recovered %s: %w", name, err)
			}
		}
	}
	return os.Remove(rollbackDir)
}

// rollbackRestore undoes a partial Restore: entries already copied from the
// profile are discarded (they were never live — the genuine pre-Restore
// state is what's sitting in rollbackDir), then every moved entry is renamed
// back from rollbackDir onto appDataPath.
func rollbackRestore(appDataPath, rollbackDir string, copied, moved []string) {
	for _, name := range copied {
		_ = os.RemoveAll(filepath.Join(appDataPath, name))
	}
	for _, name := range moved {
		dst := filepath.Join(appDataPath, name)
		_ = os.RemoveAll(dst)
		_ = os.Rename(filepath.Join(rollbackDir, name), dst)
	}
	_ = os.RemoveAll(rollbackDir)
}

// Wipe clears every non-denylisted app-data entry so the next launch starts
// a fresh, signed-out session. Denylisted entries (cache, machine/work
// state) are left untouched.
func (s *Store) Wipe(appDataPath string) error {
	entries, err := os.ReadDir(appDataPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == rollbackDirName || skip(name) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(appDataPath, name)); err != nil {
			return fmt.Errorf("wipe %s: %w", name, err)
		}
	}
	return nil
}

func HasActiveSession(appDataPath string) bool {
	return InspectCookies(filepath.Join(appDataPath, cookiesFile), time.Now()).Health == HealthUsable
}

func (s *Store) List() ([]Meta, error) {
	entries, err := os.ReadDir(s.profilesPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	profiles := make([]Meta, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		meta, err := s.loadMeta(entry.Name())
		if err != nil {
			meta = Meta{Name: entry.Name()}
		}
		meta.ObservedHealth = s.Inspect(entry.Name()).Health
		profiles = append(profiles, meta)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

// MatchLive identifies which saved profile the live app-data belongs to by
// comparing account identity (config.json's lastKnownAccountUuid), not the
// Cookies digest: the sessionKey cookie is a sliding token the app rotates
// on its own, so a live digest routinely diverges from what was last saved
// even for the correct account.
func (s *Store) MatchLive(appDataPath string) (string, Health) {
	live := filepath.Join(appDataPath, cookiesFile)
	inspection := InspectCookies(live, s.now())
	if inspection.Health != HealthUsable {
		return "", inspection.Health
	}
	liveUUID, err := readAccountUUID(appDataPath)
	if err != nil || liveUUID == "" {
		return "", HealthUnknown
	}
	profiles, err := s.List()
	if err != nil {
		return "", HealthUnknown
	}
	for _, meta := range profiles {
		if meta.AccountUUID != "" && meta.AccountUUID == liveUUID && s.Inspect(meta.Name).Health == HealthUsable {
			return meta.Name, HealthUsable
		}
	}
	return "", HealthUsable
}

// readAccountUUID reads lastKnownAccountUuid out of an app-data tree's
// config.json.
func readAccountUUID(appDataPath string) (string, error) {
	data, err := os.ReadFile(filepath.Join(appDataPath, configFile))
	if err != nil {
		return "", err
	}
	var cfg struct {
		LastKnownAccountUUID string `json:"lastKnownAccountUuid"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", err
	}
	return cfg.LastKnownAccountUUID, nil
}

func (s *Store) UpdateAccountInfo(name, email, plan string) error {
	meta, err := s.loadMeta(name)
	if err != nil {
		return err
	}
	if email != "" {
		meta.Email = email
	}
	if plan != "" {
		meta.Plan = plan
	}
	return s.saveMeta(name, meta)
}

func (s *Store) Delete(name string) error {
	if !s.Exists(name) {
		return fmt.Errorf("profile %q not found", name)
	}
	current, _ := s.Current()
	if current == name {
		_ = os.Remove(filepath.Join(s.baseDir, currentFileName))
	}
	return os.RemoveAll(s.profileDir(name))
}

func (s *Store) Current() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.baseDir, currentFileName))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *Store) SetCurrent(name string) error {
	return writeFileAtomic(filepath.Join(s.baseDir, currentFileName), []byte(name))
}

func (s *Store) setLastUsed(name string) error {
	meta, err := s.loadMeta(name)
	if err != nil {
		return err
	}
	meta.LastUsed = s.now()
	return s.saveMeta(name, meta)
}

func (s *Store) commitProfile(name, stage string) error {
	final := s.profileDir(name)
	backup := filepath.Join(s.profilesPath(), "."+name+".backup")
	_ = os.RemoveAll(backup)
	hadBackup := false
	if _, err := os.Stat(final); err == nil {
		if err := os.Rename(final, backup); err != nil {
			return err
		}
		hadBackup = true
	}
	if err := os.Rename(stage, final); err != nil {
		_ = os.Rename(backup, final)
		return err
	}
	if err := syncDir(s.profilesPath()); err != nil {
		if hadBackup {
			_ = os.RemoveAll(final)
			_ = os.Rename(backup, final)
		}
		return err
	}
	return os.RemoveAll(backup)
}

func (s *Store) recoverProfiles() error {
	entries, err := os.ReadDir(s.profilesPath())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".backup") && strings.HasPrefix(name, ".") {
			profileName := strings.TrimSuffix(strings.TrimPrefix(name, "."), ".backup")
			final := s.profileDir(profileName)
			if _, err := os.Stat(final); errors.Is(err, os.ErrNotExist) {
				if err := os.Rename(filepath.Join(s.profilesPath(), name), final); err != nil {
					return err
				}
			} else {
				_ = os.RemoveAll(filepath.Join(s.profilesPath(), name))
			}
		}
		if strings.Contains(name, ".stage-") && strings.HasPrefix(name, ".") {
			_ = os.RemoveAll(filepath.Join(s.profilesPath(), name))
		}
	}
	return nil
}

func (s *Store) ProfileCookiesPath(name string) string {
	return filepath.Join(s.profileDir(name), cookiesFile)
}

func (s *Store) profileDir(name string) string { return filepath.Join(s.profilesPath(), name) }
func (s *Store) profilesPath() string          { return filepath.Join(s.baseDir, profilesDirName) }

func (s *Store) loadMeta(name string) (Meta, error) {
	data, err := os.ReadFile(filepath.Join(s.profileDir(name), metaFile))
	if err != nil {
		return Meta{}, err
	}
	var meta Meta
	return meta, json.Unmarshal(data, &meta)
}

func (s *Store) saveMeta(name string, meta Meta) error {
	return writeJSONAtomic(filepath.Join(s.profileDir(name), metaFile), meta)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(filePerm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, filePerm)
	if err != nil {
		return err
	}
	if err := out.Chmod(filePerm); err != nil {
		out.Close()
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, dirPerm); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		from := filepath.Join(src, entry.Name())
		to := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(from, to); err != nil {
				return err
			}
			continue
		}
		if !entry.Type().IsRegular() {
			continue
		}
		if err := copyFile(from, to); err != nil {
			return err
		}
	}
	return nil
}

// validateSecureTree rejects group/other-readable files. Windows has no
// POSIX mode bits — os.FileMode there is synthesized from file attributes
// and duplicates owner bits into group/other, so the check is meaningless;
// Windows enforces access via ACLs instead, which this does not inspect.
func validateSecureTree(root string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&0077 != 0 {
			return fmt.Errorf("unsafe permissions on %s", filepath.Base(path))
		}
		return nil
	})
}

func syncTree(root string) error {
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		// Opened read-write because Windows' FlushFileBuffers (what
		// File.Sync calls) requires a write-capable handle; a read-only
		// os.Open succeeds but Sync() then fails with "Access is denied".
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			return err
		}
		defer f.Close()
		return f.Sync()
	}); err != nil {
		return err
	}
	return syncDir(root)
}

// syncDir fsyncs a directory's metadata so a prior rename/create is durable.
// Windows has no equivalent: a directory handle opened read-only cannot be
// flushed (FlushFileBuffers returns "Access is denied"), and NTFS journals
// directory changes itself, so the fsync is unnecessary there.
func syncDir(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
