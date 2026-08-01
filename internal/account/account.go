package account

// Info holds live account metadata fetched from the Claude.ai API.
type Info struct {
	Email string
	Plan  string
	// Rejected is true when the server explicitly rejected the sessionKey
	// (HTTP 401), as opposed to the lookup failing for an indeterminate
	// reason (offline, unsupported platform, transient network error).
	Rejected bool
}
