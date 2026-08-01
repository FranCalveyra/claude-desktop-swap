package profile

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestSkipCoversBothDenylistsAndLetsUnknownNamesThrough(t *testing.T) {
	for name := range cacheDenylist {
		if !skip(name) {
			t.Errorf("skip(%q) = false, want true (cache denylist)", name)
		}
	}
	for name := range workDenylist {
		if !skip(name) {
			t.Errorf("skip(%q) = false, want true (work denylist)", name)
		}
	}
	for _, name := range []string{cookiesFile, "Local Storage", "IndexedDB", "Session Storage", "config.json", "some-future-anthropic-state.json"} {
		if skip(name) {
			t.Errorf("skip(%q) = true, want false — must default to captured so future account state isn't silently lost", name)
		}
	}
}

func TestCheckpointCapturesEntireTreeExceptDenylist(t *testing.T) {
	store := newTestStore(t)
	appData := syntheticAppData(t, "live")
	mustWriteFile(t, filepath.Join(appData, "Cache", "junk"), "regenerable-cache")
	mustWriteFile(t, filepath.Join(appData, "ant-did"), "device-id")
	if err := store.Checkpoint("work", appData); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	profileDir := store.profileDir("work")
	for _, want := range []string{cookiesFile, "Local Storage", "IndexedDB", "Session Storage", "config.json", metaFile} {
		if _, err := os.Stat(filepath.Join(profileDir, want)); err != nil {
			t.Fatalf("profile missing %s: %v", want, err)
		}
	}
	for _, notWant := range []string{"Cache", "ant-did"} {
		if _, err := os.Stat(filepath.Join(profileDir, notWant)); !os.IsNotExist(err) {
			t.Fatalf("profile captured denylisted %s", notWant)
		}
	}
	assertMode(t, profileDir, dirPerm)
	assertMode(t, filepath.Join(profileDir, cookiesFile), filePerm)
	assertMode(t, filepath.Join(profileDir, metaFile), filePerm)

	meta, err := store.loadMeta("work")
	if err != nil {
		t.Fatal(err)
	}
	if meta.FormatVersion != formatVersion || meta.ObservedHealth != HealthUsable || meta.CookieDigest == "" || meta.SavedAt.IsZero() {
		t.Fatalf("incomplete v%d metadata: %+v", formatVersion, meta)
	}
	if meta.AccountUUID != "live" {
		t.Fatalf("AccountUUID = %q, want %q from config.json", meta.AccountUUID, "live")
	}
}

func TestMatchLiveIgnoresProfilesWithoutAccountUUID(t *testing.T) {
	store := newTestStore(t)
	old := store.profileDir("legacy")
	if err := os.MkdirAll(old, dirPerm); err != nil {
		t.Fatal(err)
	}
	createCookiesDB(t, filepath.Join(old, cookiesFile), ".claude.ai", "sessionKey", 0)
	mustWriteMeta(t, filepath.Join(old, metaFile), Meta{Name: "legacy", CreatedAt: time.Now(), FormatVersion: formatVersion})

	live := syntheticAppData(t, "live")
	writeConfigJSON(t, live, "")
	if name, health := store.MatchLive(live); name != "" || health != HealthUnknown {
		t.Fatalf("match = %q/%s, want unknown — live has no account identity to match on", name, health)
	}
}

func TestCheckpointRefusesUnusableLiveStateWithoutOverwritingProfile(t *testing.T) {
	store := newTestStore(t)
	healthy := syntheticAppData(t, "healthy")
	if err := store.Checkpoint("work", healthy); err != nil {
		t.Fatal(err)
	}
	before, _ := cookieDigest(filepath.Join(store.profileDir("work"), cookiesFile))

	unusable := t.TempDir()
	createCookiesDB(t, filepath.Join(unusable, cookiesFile), ".claude.ai", "other", 0)
	if err := store.Checkpoint("work", unusable); err == nil {
		t.Fatal("Checkpoint should reject missing session evidence")
	}
	after, _ := cookieDigest(filepath.Join(store.profileDir("work"), cookiesFile))
	if after != before {
		t.Fatal("unusable live state overwrote the saved usable profile")
	}
}

func TestRestoreRefusesUnsafePermissionsAndIntegrityMismatchBeforeMutation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *Store)
	}{
		{"unsafe permissions", func(t *testing.T, s *Store) {
			if runtime.GOOS == "windows" {
				t.Skip("Windows has no POSIX mode bits to make unsafe")
			}
			if err := os.Chmod(filepath.Join(s.profileDir("work"), cookiesFile), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"digest mismatch", func(t *testing.T, s *Store) {
			path := filepath.Join(s.profileDir("work"), cookiesFile)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			createCookiesDB(t, path, ".claude.ai", "sessionKey", 0)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			appData := syntheticAppData(t, "saved")
			if err := store.Checkpoint("work", appData); err != nil {
				t.Fatal(err)
			}
			live := syntheticAppData(t, "live-before")
			before := snapshotTree(t, live)
			tt.mutate(t, store)
			if err := store.Restore("work", live); err == nil {
				t.Fatal("Restore should refuse invalid profile")
			}
			after := snapshotTree(t, live)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("live tree changed before validation completed")
			}
		})
	}
}

func TestInspectRejectsFormatVersion2Profiles(t *testing.T) {
	store := newTestStore(t)
	old := store.profileDir("legacy")
	if err := os.MkdirAll(old, dirPerm); err != nil {
		t.Fatal(err)
	}
	createCookiesDB(t, filepath.Join(old, cookiesFile), ".claude.ai", "sessionKey", 0)
	mustWriteMeta(t, filepath.Join(old, metaFile), Meta{Name: "legacy", CreatedAt: time.Now(), FormatVersion: 2})

	if got := store.Inspect("legacy").Health; got != HealthUnknown {
		t.Fatalf("Inspect v2 profile = %s, want unknown", got)
	}

	live := syntheticAppData(t, "live")
	if err := store.Restore("legacy", live); err == nil {
		t.Fatal("Restore should refuse a format_version 2 profile")
	}
}

func TestExpiredProfileIsNotRepaired(t *testing.T) {
	store := newTestStore(t)
	expired := store.profileDir("expired")
	if err := os.MkdirAll(expired, dirPerm); err != nil {
		t.Fatal(err)
	}
	createCookiesDB(t, filepath.Join(expired, cookiesFile), ".claude.ai", "sessionKey", chromiumTime(store.now().Add(-time.Hour)))
	live := syntheticAppData(t, "live")
	if err := store.Restore("expired", live); err == nil {
		t.Fatal("expired profile should require reauthentication")
	}
	if _, err := store.loadMeta("expired"); err == nil {
		t.Fatal("expired profile should not gain synthesized metadata")
	}
}

func TestStoreRecoversBackupAndRemovesOrphanStage(t *testing.T) {
	base := t.TempDir()
	profiles := filepath.Join(base, profilesDirName)
	backup := filepath.Join(profiles, ".work.backup")
	stage := filepath.Join(profiles, ".work.stage-dead")
	if err := os.MkdirAll(backup, dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stage, dirPerm); err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, filepath.Join(backup, metaFile), `{}`)
	store, err := newStore(base)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Exists("work") {
		t.Fatal("backup was not recovered")
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatal("orphan stage was not removed")
	}
}

func TestRestoreMirrorsProfileAndPreservesDenylistedEntries(t *testing.T) {
	store := newTestStore(t)
	saved := syntheticAppData(t, "saved")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}

	live := syntheticAppData(t, "live")
	liveOnly := filepath.Join(live, "Preferences")
	mustWriteFile(t, liveOnly, "stale-live-only")
	denylisted := map[string]string{
		"ant-did":                      "device-id",
		"claude_desktop_config.json":   "mcp-config",
		filepath.Join("Cache", "junk"): "regenerable-cache",
	}
	for path, content := range denylisted {
		mustWriteFile(t, filepath.Join(live, path), content)
	}

	if err := store.Restore("work", live); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, err := os.Stat(liveOnly); !os.IsNotExist(err) {
		t.Fatal("live-only entry should be removed by the mirror restore")
	}
	got, err := os.ReadFile(filepath.Join(live, configFile))
	want := fmt.Sprintf(`{"lastKnownAccountUuid":%q}`, "saved")
	if err != nil || string(got) != want {
		t.Fatalf("config.json not mirrored from profile: %q %v", got, err)
	}
	for path, want := range denylisted {
		got, err := os.ReadFile(filepath.Join(live, path))
		if err != nil || string(got) != want {
			t.Fatalf("denylisted %s changed: %q %v", path, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(live, rollbackDirName)); !os.IsNotExist(err) {
		t.Fatal("rollback backup should not remain after a successful restore")
	}
}

func TestRestoreFailureMidCopyLeavesTreeUnchanged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX mode bits to make a file unreadable")
	}
	store := newTestStore(t)
	saved := syntheticAppData(t, "saved")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}
	// "Cookies" and "IndexedDB" sort before "Local Storage", so both are
	// already copied by the time this unreadable file is hit — a genuine
	// mid-copy failure, not a rejection at the Inspect gate.
	poisoned := filepath.Join(store.profileDir("work"), "Local Storage", "leveldb", "CURRENT")
	if err := os.Chmod(poisoned, 0000); err != nil {
		t.Fatal(err)
	}

	live := syntheticAppData(t, "live")
	before := snapshotTree(t, live)

	if err := store.Restore("work", live); err == nil {
		t.Fatal("Restore should fail when a profile entry cannot be read")
	}

	after := snapshotTree(t, live)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("live tree changed after a failed restore:\nbefore: %v\nafter:  %v", before, after)
	}
	if _, err := os.Stat(filepath.Join(live, rollbackDirName)); !os.IsNotExist(err) {
		t.Fatal("rollback backup should not remain after a failed restore")
	}
}

func TestRestoreTrackingFailureRollsBackCookies(t *testing.T) {
	store := newTestStore(t)
	saved := syntheticAppData(t, "saved")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}
	live := syntheticAppData(t, "live")
	before, _ := cookieDigest(filepath.Join(live, cookiesFile))
	if err := os.Mkdir(filepath.Join(store.baseDir, currentFileName), dirPerm); err != nil {
		t.Fatal(err)
	}

	if err := store.Restore("work", live); err == nil {
		t.Fatal("Restore should fail when tracking cannot commit")
	}
	after, _ := cookieDigest(filepath.Join(live, cookiesFile))
	if after != before {
		t.Fatal("live Cookies were not rolled back")
	}
	if _, err := os.Stat(filepath.Join(live, rollbackDirName)); !os.IsNotExist(err) {
		t.Fatal("rollback backup should not remain after a failed restore")
	}
}

func TestMatchLiveIdentifiesByAccountUUIDEvenIfCookiesRotated(t *testing.T) {
	store := newTestStore(t)
	saved := syntheticAppData(t, "work")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}

	// Same account (UUID), but the sessionKey cookie rotated since the save —
	// the sliding 28-day token the app renews on its own.
	live := syntheticAppData(t, "rotated-session")
	writeConfigJSON(t, live, "work")

	if name, health := store.MatchLive(live); name != "work" || health != HealthUsable {
		t.Fatalf("match = %q/%s, want work/usable", name, health)
	}
}

func TestMatchLiveDoesNotTrustStaleCurrent(t *testing.T) {
	store := newTestStore(t)
	a := syntheticAppData(t, "a")
	b := syntheticAppData(t, "b")
	if err := store.Checkpoint("a", a); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCurrent("a"); err != nil {
		t.Fatal(err)
	}
	if name, health := store.MatchLive(b); name != "" || health != HealthUsable {
		t.Fatalf("match = %q/%s, want unknown usable", name, health)
	}
	if name, health := store.MatchLive(a); name != "a" || health != HealthUsable {
		t.Fatalf("match = %q/%s, want a/usable", name, health)
	}
}

func TestWipeRemovesAccountStateButPreservesDenylist(t *testing.T) {
	store := newTestStore(t)
	appData := syntheticAppData(t, "live")
	preserved := map[string]string{"ant-did": "device-id", "claude_desktop_config.json": "mcp-config"}
	for path, content := range preserved {
		mustWriteFile(t, filepath.Join(appData, path), content)
	}
	if err := store.Wipe(appData); err != nil {
		t.Fatal(err)
	}
	for path, want := range preserved {
		got, err := os.ReadFile(filepath.Join(appData, path))
		if err != nil || string(got) != want {
			t.Fatalf("denylisted %s was touched: %q %v", path, got, err)
		}
	}
	for _, path := range []string{cookiesFile, "config.json", "Local Storage"} {
		if _, err := os.Stat(filepath.Join(appData, path)); !os.IsNotExist(err) {
			t.Fatalf("%s was not wiped", path)
		}
	}
}

func TestProfileCookiesPathResolvesUnderProfile(t *testing.T) {
	store := newTestStore(t)
	got := store.ProfileCookiesPath("work")
	want := filepath.Join(store.profileDir("work"), cookiesFile)
	if got != want {
		t.Fatalf("ProfileCookiesPath = %q, want %q", got, want)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := newStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store.now = func() time.Time { return time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC) }
	return store
}

func syntheticAppData(t *testing.T, marker string) string {
	t.Helper()
	dir := t.TempDir()
	createCookiesDBWithMarker(t, filepath.Join(dir, cookiesFile), marker)
	for _, path := range []string{
		filepath.Join("Local Storage", "leveldb", "CURRENT"),
		filepath.Join("IndexedDB", "data"),
		filepath.Join("Session Storage", "data"),
	} {
		mustWriteFile(t, filepath.Join(dir, path), marker)
	}
	writeConfigJSON(t, dir, marker)
	return dir
}

// writeConfigJSON writes a minimal config.json whose lastKnownAccountUuid is
// accountUUID, mirroring the one real field MatchLive reads.
func writeConfigJSON(t *testing.T, appDataPath, accountUUID string) {
	t.Helper()
	mustWriteFile(t, filepath.Join(appDataPath, configFile), fmt.Sprintf(`{"lastKnownAccountUuid":%q}`, accountUUID))
}

func createCookiesDBWithMarker(t *testing.T, path, marker string) {
	t.Helper()
	db := openSQLite(t, path)
	mustExec(t, db, `CREATE TABLE cookies (host_key TEXT, name TEXT, expires_utc INTEGER, value TEXT, encrypted_value BLOB)`)
	mustExec(t, db, `CREATE TABLE fixture_marker (marker TEXT)`)
	mustExec(t, db, `INSERT INTO cookies(host_key, name, expires_utc, value, encrypted_value) VALUES ('.claude.ai', 'sessionKey', 0, 'secret', x'0102')`)
	mustExec(t, db, `INSERT INTO fixture_marker VALUES (?)`, marker)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), dirPerm); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), filePerm); err != nil {
		t.Fatal(err)
	}
}

func mustWriteMeta(t *testing.T, path string, meta Meta) {
	t.Helper()
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}
	mustWriteFile(t, path, string(data))
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != want {
		t.Fatalf("%s mode = %o, want %o", path, info.Mode().Perm(), want)
	}
}

// snapshotTree records the relative path and content of every regular file
// under root, skipping the rollback backup directory itself.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == rollbackDirName {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		snap[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}
