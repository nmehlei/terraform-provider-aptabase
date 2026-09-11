// Package aptabase is a minimal, hand-written HTTP client for an
// aptabase-plus instance's /api/v0 management API. It intentionally has
// no dependency on the Terraform Plugin Framework so it can be used and
// tested standalone.
package aptabase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	managementPrefix = "api/v0/"
	maxResponseBody  = 1 << 20 // 1 MiB
)

// Config configures a Client.
type Config struct {
	// Endpoint is the base URL of the aptabase-plus instance, e.g.
	// "https://aptabase.example.com". Must be HTTPS unless AllowInsecure
	// is set.
	Endpoint string

	// Token authenticates every request as a bearer token (an
	// aptabase-plus API key). The client does not read this from the
	// environment; callers source it from APTABASE_TOKEN or a sensitive
	// provider attribute themselves.
	Token string

	// Timeout bounds every HTTP request. Defaults to 30s when zero.
	Timeout time.Duration

	// AllowInsecure permits a plain-HTTP endpoint. Intended only for
	// local development against a disposable instance.
	AllowInsecure bool

	// HTTPClient overrides the underlying HTTP client. Primarily for tests.
	HTTPClient *http.Client
}

// Client is an aptabase-plus management API client.
type Client struct {
	baseURL    *url.URL
	token      string
	timeout    time.Duration
	httpClient *http.Client
}

// New validates cfg and returns a ready Client. It performs no network
// call - aptabase-plus has no schema/version endpoint to check against,
// unlike Bugsink.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, &ConfigError{Msg: "endpoint is required"}
	}
	if cfg.Token == "" {
		return nil, &ConfigError{Msg: "token is required"}
	}

	u, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, &ConfigError{Msg: "invalid endpoint", Err: err}
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !cfg.AllowInsecure {
			return nil, &ConfigError{Msg: "endpoint uses plain HTTP; set AllowInsecure for local development only"}
		}
	default:
		return nil, &ConfigError{Msg: fmt.Sprintf("unsupported endpoint scheme %q", u.Scheme)}
	}

	base := *u
	base.User = nil // never let a credential embedded in the endpoint reach a request URL or error message
	base.RawQuery = ""
	base.Fragment = ""
	if !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	base.Path += managementPrefix

	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		baseURL:    &base,
		token:      cfg.Token,
		timeout:    timeout,
		httpClient: httpClient,
	}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, out any) error {
	reqURL, err := c.resolve(path)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "terraform-provider-aptabase-client")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("aptabase: %s %s: %w", method, reqURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return fmt.Errorf("aptabase: %s %s: reading response: %w", method, reqURL, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Method: method, URL: reqURL, StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("aptabase: %s %s: decoding response (status %d, content-type %q): %w",
			method, reqURL, resp.StatusCode, resp.Header.Get("Content-Type"), err)
	}
	return nil
}

// resolve turns a path relative to the management API root into a full
// request URL.
func (c *Client) resolve(path string) (string, error) {
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("aptabase: invalid path %q: %w", path, err)
	}
	return c.baseURL.ResolveReference(ref).String(), nil
}
