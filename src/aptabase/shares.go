package aptabase

import (
	"context"
	"net/http"
	"net/url"
)

// Share is an email-based app share.
type Share struct {
	Email     string `json:"email"`
	CreatedAt string `json:"createdAt"`
}

func (c *Client) ListShares(ctx context.Context, appID string) ([]Share, error) {
	var shares []Share
	if err := c.do(ctx, http.MethodGet, appsPath+appID+"/shares", nil, &shares); err != nil {
		return nil, err
	}
	return shares, nil
}

func (c *Client) AddShare(ctx context.Context, appID, email string) error {
	return c.do(ctx, http.MethodPut, appsPath+appID+"/shares/"+url.PathEscape(email), nil, nil)
}

func (c *Client) RemoveShare(ctx context.Context, appID, email string) error {
	return c.do(ctx, http.MethodDelete, appsPath+appID+"/shares/"+url.PathEscape(email), nil, nil)
}
