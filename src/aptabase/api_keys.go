package aptabase

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

const apiKeysPath = "api-keys/"

// ApiKeySummary is an API key's metadata, never its plaintext secret.
type ApiKeySummary struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"keyPrefix"`
	LastUsedAt *string `json:"lastUsedAt"`
	ExpiresAt  *string `json:"expiresAt"`
	CreatedAt  string  `json:"createdAt"`
}

// ApiKeyCreated additionally carries the plaintext key. The server only
// ever returns this once, at creation.
type ApiKeyCreated struct {
	ApiKeySummary
	Key string `json:"key"`
}

type createApiKeyRequest struct {
	Name      string  `json:"name"`
	ExpiresAt *string `json:"expiresAt,omitempty"`
}

func (c *Client) ListApiKeys(ctx context.Context) ([]ApiKeySummary, error) {
	var keys []ApiKeySummary
	if err := c.do(ctx, http.MethodGet, apiKeysPath, nil, &keys); err != nil {
		return nil, err
	}
	return keys, nil
}

// CreateApiKey creates a key. expiresAt, when non-nil, must be an RFC3339
// timestamp string.
func (c *Client) CreateApiKey(ctx context.Context, name string, expiresAt *string) (*ApiKeyCreated, error) {
	body, err := json.Marshal(createApiKeyRequest{Name: name, ExpiresAt: expiresAt})
	if err != nil {
		return nil, err
	}
	var out ApiKeyCreated
	if err := c.do(ctx, http.MethodPost, apiKeysPath, bytes.NewReader(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteApiKey(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, apiKeysPath+id, nil, nil)
}
