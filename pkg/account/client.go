// Package account is the HTTP client for 9itchi-account-api. Base URL comes from env, never hard-coded.
package account

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

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 50 * time.Second},
	}
}

// User is the minimal user view served by GET /internal/users/:id.
type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// GetUser fetches a user by UUID through GET /internal/users/:id.
func (c *Client) GetUser(ctx context.Context, userID string) (*User, error) {
	path := strings.TrimRight(c.BaseURL, "/") + "/internal/users/" + url.PathEscape(userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create get user request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("get user returned status %d", resp.StatusCode)
	}
	var u User
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("decode user: %w", err)
	}
	return &u, nil
}
