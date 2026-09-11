package aptabase

import "fmt"

// ConfigError indicates the Client was misconfigured (bad endpoint,
// missing token). It never includes the token.
type ConfigError struct {
	Msg string
	Err error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("aptabase: %s: %v", e.Msg, e.Err)
	}
	return "aptabase: " + e.Msg
}

func (e *ConfigError) Unwrap() error { return e.Err }

// APIError represents a non-2xx response from the aptabase-plus
// management API. Built only from method/URL/status/body, never from
// request headers, so it never includes the bearer token.
type APIError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	body := e.Body
	const maxErrBody = 2048
	if len(body) > maxErrBody {
		body = body[:maxErrBody] + "... (truncated)"
	}
	return fmt.Sprintf("aptabase: %s %s: unexpected status %d: %s", e.Method, e.URL, e.StatusCode, body)
}

// NotFound reports whether the error is a 404, the condition under which
// callers should treat a resource as removed from the remote and drop it
// from Terraform state instead of surfacing a hard error.
func (e *APIError) NotFound() bool {
	return e.StatusCode == 404
}
