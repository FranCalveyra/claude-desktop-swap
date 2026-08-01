package profile

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestInspectCookiesClassifiesSyntheticSessions(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		prepare func(*testing.T, string)
		want    Health
	}{
		{"usable persistent", func(t *testing.T, p string) {
			createCookiesDB(t, p, ".claude.ai", "sessionKey", chromiumTime(now.Add(time.Hour)))
		}, HealthUsable},
		{"usable session cookie", func(t *testing.T, p string) { createCookiesDB(t, p, ".claude.ai", "sessionKey", 0) }, HealthUsable},
		{"expired", func(t *testing.T, p string) { createCookiesDB(t, p, ".claude.ai", "sessionKey", chromiumTime(now)) }, HealthExpired},
		{"missing row", func(t *testing.T, p string) {
			createCookiesDB(t, p, ".example.com", "other", chromiumTime(now.Add(time.Hour)))
		}, HealthMissing},
		{"missing file", func(t *testing.T, p string) {}, HealthMissing},
		{"unsupported schema", func(t *testing.T, p string) {
			db := openSQLite(t, p)
			mustExec(t, db, `CREATE TABLE cookies (host_key TEXT, name TEXT)`)
			db.Close()
		}, HealthUnknown},
		{"corrupt", func(t *testing.T, p string) { mustWriteFile(t, p, "not sqlite") }, HealthUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), cookiesFile)
			tt.prepare(t, path)
			got := InspectCookies(path, now)
			if got.Health != tt.want {
				t.Fatalf("health = %q (%s), want %q", got.Health, got.Reason, tt.want)
			}
		})
	}
}

func TestCheckpointCookiesFlushesWAL(t *testing.T) {
	path := filepath.Join(t.TempDir(), cookiesFile)
	db := openSQLite(t, path)
	mustExec(t, db, `PRAGMA journal_mode=WAL`)
	mustExec(t, db, `CREATE TABLE cookies (host_key TEXT, name TEXT, expires_utc INTEGER)`)
	mustExec(t, db, `INSERT INTO cookies VALUES ('.claude.ai', 'sessionKey', 0)`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := CheckpointCookies(path); err != nil {
		t.Fatalf("CheckpointCookies: %v", err)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		t.Fatalf("WAL size = %d, want 0 after checkpoint", info.Size())
	}
	if got := InspectCookies(path, time.Now()); got.Health != HealthUsable {
		t.Fatalf("health after checkpoint = %q (%s)", got.Health, got.Reason)
	}
}

func TestStripVolatileCookiesRemovesOnlyBlocklisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), cookiesFile)
	db := openSQLite(t, path)
	mustExec(t, db, `PRAGMA journal_mode=WAL`)
	mustExec(t, db, `CREATE TABLE cookies (host_key TEXT, name TEXT, expires_utc INTEGER)`)
	mustExec(t, db, `INSERT INTO cookies VALUES ('.claude.ai', 'sessionKey', 0)`)
	mustExec(t, db, `INSERT INTO cookies VALUES ('.claude.ai', 'cf_clearance', 0)`)
	mustExec(t, db, `INSERT INTO cookies VALUES ('.claude.ai', '__cf_bm', 0)`)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if err := StripVolatileCookies(path); err != nil {
		t.Fatalf("StripVolatileCookies: %v", err)
	}

	db = openSQLite(t, path)
	defer db.Close()
	var names []string
	rows, err := db.Query(`SELECT name FROM cookies ORDER BY name`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	if len(names) != 1 || names[0] != "sessionKey" {
		t.Fatalf("remaining cookies = %v, want [sessionKey]", names)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		t.Fatalf("WAL size = %d, want 0 after strip", info.Size())
	}
}

func TestInspectCookiesReturnsUnknownWhenDatabaseIsLocked(t *testing.T) {
	path := filepath.Join(t.TempDir(), cookiesFile)
	createCookiesDB(t, path, ".claude.ai", "sessionKey", 0)

	db := openSQLite(t, path)
	defer db.Close()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `BEGIN EXCLUSIVE`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := conn.ExecContext(context.Background(), `ROLLBACK`); err != nil {
			t.Errorf("rollback lock: %v", err)
		}
	}()

	started := time.Now()
	got := inspectCookiesWithTimeout(path, time.Now(), 50*time.Millisecond)
	if got.Health != HealthUnknown {
		t.Fatalf("health = %q (%s), want unknown", got.Health, got.Reason)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("inspection blocked for %s", elapsed)
	}
}

func createCookiesDB(t *testing.T, path, host, name string, expires int64) {
	t.Helper()
	db := openSQLite(t, path)
	mustExec(t, db, `CREATE TABLE cookies (host_key TEXT, name TEXT, expires_utc INTEGER, value TEXT, encrypted_value BLOB)`)
	mustExec(t, db, `INSERT INTO cookies(host_key, name, expires_utc, value, encrypted_value) VALUES (?, ?, ?, 'secret', x'0102')`, host, name, expires)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, filePerm); err != nil {
		t.Fatal(err)
	}
}

func openSQLite(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func chromiumTime(t time.Time) int64 {
	return t.UnixMicro() + 11644473600000000
}

func TestClassifyRenewal(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name      string
		expiresAt time.Time
		want      Renewal
	}{
		{"no evidence", time.Time{}, RenewalUnknown},
		{"already lapsed", now.Add(-time.Hour), RenewalExpired},
		{"exactly now", now, RenewalExpired},
		{"edge of window", now.Add(RenewalWindow), RenewalSoon},
		{"inside window", now.Add(3 * 24 * time.Hour), RenewalSoon},
		{"just outside window", now.Add(RenewalWindow + time.Minute), RenewalOK},
		{"fresh sliding token", now.Add(28 * 24 * time.Hour), RenewalOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyRenewal(tc.expiresAt, now); got != tc.want {
				t.Fatalf("ClassifyRenewal = %s, want %s", got, tc.want)
			}
		})
	}
}

func TestInspectCookiesCarriesExpiry(t *testing.T) {
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	expiry := now.Add(21 * 24 * time.Hour)
	path := filepath.Join(t.TempDir(), "Cookies")
	createCookiesDB(t, path, ".claude.ai", "sessionKey", chromiumTime(expiry))

	got := InspectCookies(path, now)
	if got.Health != HealthUsable {
		t.Fatalf("health = %s, want usable", got.Health)
	}
	if !got.ExpiresAt.Equal(expiry) {
		t.Fatalf("ExpiresAt = %s, want %s", got.ExpiresAt, expiry)
	}
}

// A non-persistent session cookie carries no deadline; reporting one would
// invent a warning out of missing data.
func TestInspectCookiesLeavesExpiryZeroForSessionCookie(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Cookies")
	createCookiesDB(t, path, ".claude.ai", "sessionKey", 0)

	got := InspectCookies(path, time.Now())
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt = %s, want zero", got.ExpiresAt)
	}
	if ClassifyRenewal(got.ExpiresAt, time.Now()) != RenewalUnknown {
		t.Fatal("session cookie must classify as unknown, not expiring")
	}
}
