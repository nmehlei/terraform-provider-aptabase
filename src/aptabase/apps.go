package aptabase

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

const appsPath = "apps/"

// App is an aptabase-plus application. AppKey is the SDK ingestion key,
// distinct from any management API key.
type App struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	IconPath     string `json:"iconPath"`
	AppKey       string `json:"appKey"`
	HasOwnership bool   `json:"hasOwnership"`
	HasEvents    bool   `json:"hasEvents"`
}

type createAppRequest struct {
	Name string `json:"name"`
}

// ListApps returns every app owned by or shared with the authenticated user.
func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	var apps []App
	if err := c.do(ctx, http.MethodGet, appsPath, nil, &apps); err != nil {
		return nil, err
	}
	return apps, nil
}

// CreateApp creates an app. The management API does not accept an icon at
// creation time - use UpdateApp afterward to set one.
func (c *Client) CreateApp(ctx context.Context, name string) (*App, error) {
	body, err := json.Marshal(createAppRequest{Name: name})
	if err != nil {
		return nil, err
	}
	var out App
	if err := c.do(ctx, http.MethodPost, appsPath, bytes.NewReader(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetApp retrieves an app by ID. Returns an *APIError with NotFound()
// true if the app doesn't exist or isn't visible to the caller.
func (c *Client) GetApp(ctx context.Context, id string) (*App, error) {
	var out App
	if err := c.do(ctx, http.MethodGet, appsPath+id, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type updateAppRequest struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// UpdateApp sets the app's name and, when iconBase64 is non-empty,
// replaces its icon. Pass an empty iconBase64 to leave the icon
// unchanged - the server only updates the icon when the field is
// non-empty.
func (c *Client) UpdateApp(ctx context.Context, id, name, iconBase64 string) (*App, error) {
	body, err := json.Marshal(updateAppRequest{Name: name, Icon: iconBase64})
	if err != nil {
		return nil, err
	}
	var out App
	if err := c.do(ctx, http.MethodPut, appsPath+id, bytes.NewReader(body), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteApp soft-deletes an app. Unlike Bugsink, this is a real, working
// delete.
func (c *Client) DeleteApp(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, appsPath+id, nil, nil)
}
