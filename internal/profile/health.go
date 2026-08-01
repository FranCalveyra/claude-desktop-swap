package profile

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Health string

const (
	HealthUsable  Health = "usable"
	HealthExpired Health = "expired"
	HealthMissing Health = "missing"
	HealthUnknown Health = "unknown"
)

type Inspection struct {
	Health Health
	Reason string
	// ExpiresAt is the sessionKey cookie's expiry. Zero means "no deadline
	// known" — a non-persistent cookie or an unreadable database — never
	// "expires now". It is deliberately not a Health value: a session
	// expiring in two days is still fully usable, and the Health guards in
	// save/use would refuse it.
	ExpiresAt time.Time
}

// RenewalWindow is how close to expiry a session starts being reported as
// needing a re-login. The sessionKey is a sliding ~28-day token the app
// renews whenever that account is used, so reaching this window means the
// profile has gone roughly three weeks untouched.
const RenewalWindow = 7 * 24 * time.Hour

type Renewal string

const (
	RenewalOK      Renewal = "ok"
	RenewalSoon    Renewal = "soon"
	RenewalExpired Renewal = "expired"
	RenewalUnknown Renewal = "unknown"
)

func ClassifyRenewal(expiresAt, now time.Time) Renewal {
	if expiresAt.IsZero() {
		return RenewalUnknown
	}
	if !expiresAt.After(now) {
		return RenewalExpired
	}
	if expiresAt.Sub(now) <= RenewalWindow {
		return RenewalSoon
	}
	return RenewalOK
}

const chromiumEpochOffsetMicros int64 = 11644473600000000

const inspectionTimeout = 500 * time.Millisecond

// sqlitePath converts an OS path into the form net/url.URL expects for a
// file: scheme, so a Windows drive letter (e.g. C:\Users\...) isn't parsed
// as a URL host.
func sqlitePath(path string) string {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed
	}
	return slashed
}

func InspectCookies(path string, now time.Time) Inspection {
	return inspectCookiesWithTimeout(path, now, inspectionTimeout)
}

func inspectCookiesWithTimeout(path string, now time.Time, timeout time.Duration) Inspection {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return Inspection{Health: HealthMissing, Reason: "Cookies database is missing"}
	} else if err != nil {
		return Inspection{Health: HealthUnknown, Reason: "Cookies database cannot be inspected"}
	}

	dsn := &url.URL{Scheme: "file", Path: sqlitePath(path)}
	query := dsn.Query()
	query.Set("mode", "ro")
	query.Add("_pragma", fmt.Sprintf("busy_timeout(%d)", max(timeout.Milliseconds(), 1)))
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite", dsn.String())
	if err != nil {
		return Inspection{Health: HealthUnknown, Reason: "Cookies database cannot be opened"}
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var check string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check); err != nil || check != "ok" {
		return Inspection{Health: HealthUnknown, Reason: "Cookies database failed integrity inspection"}
	}

	var expires int64
	err = db.QueryRowContext(ctx, `SELECT expires_utc FROM cookies WHERE host_key IN ('.claude.ai', 'claude.ai') AND name = 'sessionKey' LIMIT 1`).Scan(&expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Inspection{Health: HealthMissing, Reason: "Claude session cookie is missing"}
	}
	if err != nil {
		return Inspection{Health: HealthUnknown, Reason: "Cookies schema is unsupported"}
	}
	var expiresAt time.Time
	if expires > 0 {
		expiresAt = time.UnixMicro(expires - chromiumEpochOffsetMicros).UTC()
	}
	if expires > 0 && expires-chromiumEpochOffsetMicros <= now.UnixMicro() {
		return Inspection{Health: HealthExpired, Reason: "Claude session has expired; reauthentication is required", ExpiresAt: expiresAt}
	}
	return Inspection{Health: HealthUsable, Reason: "Claude session evidence is locally usable", ExpiresAt: expiresAt}
}

// volatileCookieNames are device/IP-bound Cloudflare cookies that hard-fail
// when a stale snapshot value is injected. Deleting them lets Cloudflare
// reissue fresh ones on first load instead of rejecting the request.
var volatileCookieNames = []string{"cf_clearance", "__cf_bm"}

// StripVolatileCookies deletes the volatile cookies from a Cookies DB and
// folds the change into the main file via a TRUNCATE checkpoint, so a later
// removal of the -wal sidecar cannot discard the delete.
func StripVolatileCookies(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(volatileCookieNames)), ",")
	args := make([]any, len(volatileCookieNames))
	for i, name := range volatileCookieNames {
		args[i] = name
	}
	if _, err := db.Exec(`DELETE FROM cookies WHERE name IN (`+placeholders+`)`, args...); err != nil {
		return err
	}
	var busy, logFrames, checkpointed int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	return nil
}

func CheckpointCookies(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var busy, logFrames, checkpointed int
	if err := db.QueryRow(`PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return err
	}
	if busy != 0 {
		return fmt.Errorf("cookies WAL remains busy")
	}
	return nil
}

func cookieDigest(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
