package account

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetJSONReturnsErrUnauthorizedOn401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	_, err := getJSON[accountResponse](server.URL, "stale-session-key")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want ErrUnauthorized", err)
	}
}

func TestGetJSONReturnsGenericErrorOnOtherStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := getJSON[accountResponse](server.URL, "any-key")
	if err == nil || errors.Is(err, ErrUnauthorized) {
		t.Fatalf("err = %v, want a non-ErrUnauthorized error", err)
	}
}
