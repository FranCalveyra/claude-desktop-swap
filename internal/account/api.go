package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnauthorized indicates the server explicitly rejected the sessionKey
// (HTTP 401), distinct from an indeterminate failure (offline, timeout).
var ErrUnauthorized = errors.New("session rejected by server")

const (
	orgsEndpoint    = "https://claude.ai/api/organizations"
	accountEndpoint = "https://claude.ai/api/account"
	userAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var defaultClient = &http.Client{Timeout: 10 * time.Second}

// orgResponse covers the known shapes of the /api/organizations response.
type orgResponse struct {
	Capabilities json.RawMessage `json:"capabilities"`
	Settings     struct {
		Tier string `json:"tier"`
	} `json:"settings"`
	BillingInfo struct {
		Tier string `json:"tier"`
		Plan string `json:"plan"`
	} `json:"billing_info"`
}

type accountResponse struct {
	Email string `json:"email_address"`
}

// fetchFromAPI calls the Claude.ai API using a decrypted sessionKey cookie and
// returns the best-effort email and plan for the account.
func fetchFromAPI(sessionKey string) (Info, error) {
	info := Info{}

	// The account holds several orgs (chat, API, ...); take the first that
	// resolves to a subscription plan.
	orgs, err := getJSON[[]orgResponse](orgsEndpoint, sessionKey)
	if errors.Is(err, ErrUnauthorized) {
		info.Rejected = true
	} else if err == nil {
		for _, org := range orgs {
			if plan := parsePlan(org); plan != "" {
				info.Plan = plan
				break
			}
		}
	}

	acc, err := getJSON[accountResponse](accountEndpoint, sessionKey)
	if errors.Is(err, ErrUnauthorized) {
		info.Rejected = true
	} else if err == nil {
		info.Email = acc.Email
	}

	return info, nil
}

// getJSON performs an authenticated GET request and unmarshals the JSON body into T.
func getJSON[T any](url, sessionKey string) (T, error) {
	var zero T
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return zero, err
	}
	req.Header.Set("Cookie", "sessionKey="+sessionKey)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://claude.ai/")

	resp, err := defaultClient.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return zero, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, err
	}
	var result T
	return result, json.Unmarshal(body, &result)
}
