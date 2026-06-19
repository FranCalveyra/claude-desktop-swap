package profile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCheckpointCreatesMinimalSecureV2Profile(t *testing.T) {
	appData := syntheticAppData(t, "live")
	store := newTestStore(t)
	if err := store.Checkpoint("work", appData); err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}

	entries, err := os.ReadDir(store.profileDir("work"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].Name() != cookiesFile || entries[1].Name() != metaFile {
		t.Fatalf("profile artifacts = %v, want only Cookies and meta.json", entryNames(entries))
	}
	assertMode(t, store.profileDir("work"), dirPerm)
	assertMode(t, filepath.Join(store.profileDir("work"), cookiesFile), filePerm)
	assertMode(t, filepath.Join(store.profileDir("work"), metaFile), filePerm)
	meta, err := store.loadMeta("work")
	if err != nil {
		t.Fatal(err)
	}
	if meta.FormatVersion != 2 || meta.ObservedHealth != HealthUsable || meta.CookieDigest == "" || meta.SavedAt.IsZero() {
		t.Fatalf("incomplete v2 metadata: %+v", meta)
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
			before, _ := cookieDigest(filepath.Join(live, cookiesFile))
			tt.mutate(t, store)
			if err := store.Restore("work", live); err == nil {
				t.Fatal("Restore should refuse invalid profile")
			}
			after, _ := cookieDigest(filepath.Join(live, cookiesFile))
			if after != before {
				t.Fatal("live Cookies changed before validation completed")
			}
		})
	}
}

func TestHealthyV1RestoresAndMigratesOnlyOnCheckpoint(t *testing.T) {
	store := newTestStore(t)
	v1 := store.profileDir("legacy")
	if err := os.MkdirAll(filepath.Join(v1, "leveldb"), dirPerm); err != nil {
		t.Fatal(err)
	}
	createCookiesDB(t, filepath.Join(v1, cookiesFile), ".claude.ai", "sessionKey", 0)
	mustWriteFile(t, filepath.Join(v1, "leveldb", "legacy"), "keep-until-commit")
	mustWriteMeta(t, filepath.Join(v1, metaFile), Meta{Name: "legacy", CreatedAt: time.Now()})
	live := syntheticAppData(t, "other")

	if err := store.Restore("legacy", live); err != nil {
		t.Fatalf("restore v1: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v1, "leveldb", "legacy")); err != nil {
		t.Fatal("restore eagerly migrated v1")
	}
	if err := store.Checkpoint("legacy", live); err != nil {
		t.Fatalf("checkpoint migrated v1: %v", err)
	}
	if _, err := os.Stat(filepath.Join(v1, "leveldb")); !os.IsNotExist(err) {
		t.Fatal("legacy payload remains after v2 commit")
	}
	meta, _ := store.loadMeta("legacy")
	if meta.FormatVersion != 2 {
		t.Fatalf("format version = %d", meta.FormatVersion)
	}
}

func TestUnusableV1IsNotRepaired(t *testing.T) {
	store := newTestStore(t)
	v1 := store.profileDir("expired")
	if err := os.MkdirAll(v1, dirPerm); err != nil {
		t.Fatal(err)
	}
	createCookiesDB(t, filepath.Join(v1, cookiesFile), ".claude.ai", "sessionKey", chromiumTime(store.now().Add(-time.Hour)))
	live := syntheticAppData(t, "live")
	if err := store.Restore("expired", live); err == nil {
		t.Fatal("expired v1 should require reauthentication")
	}
	meta, _ := store.loadMeta("expired")
	if meta.FormatVersion == 2 {
		t.Fatal("expired v1 was synthesized as v2")
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

func TestRestoreReplacesCookiesClearsVolatileAndPreservesGlobalState(t *testing.T) {
	store := newTestStore(t)
	saved := syntheticAppData(t, "saved")
	if err := store.Checkpoint("work", saved); err != nil {
		t.Fatal(err)
	}
	live := syntheticAppData(t, "live")
	for _, name := range []string{cookiesJournalFile, cookiesWALFile, cookiesSHMFile} {
		mustWriteFile(t, filepath.Join(live, name), "stale")
	}
	global := map[string]string{"config.json": "global", "WebStorage/state": "web", "partitions/p": "partition", deviceIDFile: "machine"}
	for path, content := range global {
		mustWriteFile(t, filepath.Join(live, path), content)
	}

	if err := store.Restore("work", live); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	for _, path := range []string{filepath.Join(live, localStorageDir, leveldbDir), filepath.Join(live, indexedDBDir), filepath.Join(live, sessionStorageDir)} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("volatile path remains: %s", path)
		}
	}
	for _, name := range []string{cookiesJournalFile, cookiesWALFile, cookiesSHMFile} {
		if _, err := os.Stat(filepath.Join(live, name)); !os.IsNotExist(err) {
			t.Fatalf("sidecar remains: %s", name)
		}
	}
	for path, want := range global {
		got, err := os.ReadFile(filepath.Join(live, path))
		if err != nil || string(got) != want {
			t.Fatalf("global %s changed: %q %v", path, got, err)
		}
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

func TestWipePreservesMachineAndGlobalFiles(t *testing.T) {
	store := newTestStore(t)
	appData := syntheticAppData(t, "live")
	for path, content := range map[string]string{deviceIDFile: "device", "config.json": "config"} {
		mustWriteFile(t, filepath.Join(appData, path), content)
	}
	if err := store.Wipe(appData); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{deviceIDFile, "config.json"} {
		if _, err := os.Stat(filepath.Join(appData, path)); err != nil {
			t.Fatalf("%s was removed", path)
		}
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
	for _, path := range []string{filepath.Join(localStorageDir, leveldbDir, "CURRENT"), filepath.Join(indexedDBDir, "data"), filepath.Join(sessionStorageDir, "data")} {
		mustWriteFile(t, filepath.Join(dir, path), marker)
	}
	return dir
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

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, entry := range entries {
		names[i] = entry.Name()
	}
	return names
}
