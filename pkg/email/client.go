// Package email is the HTTP client for 9itchi-email-api. Base URL comes from env, never hard-coded.
package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNotFound means the email was sent for no application.
var ErrNotFound = errors.New("email context not found")

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

// Context is what an email was sent for, as served by GET /internal/emails/:id/context.
type Context struct {
	EmailID       string `json:"email_id"`
	UserID        string `json:"user_id"`
	ApplicationID string `json:"application_id"`
	JobID         string `json:"job_id"`
	JobTitle      string `json:"job_title"`
	Company       string `json:"company"`
}

// GetContext fetches the application an email was sent for (ErrNotFound when there is none).
func (c *Client) GetContext(ctx context.Context, emailID string) (*Context, error) {
	path := strings.TrimRight(c.BaseURL, "/") + "/internal/emails/" + url.PathEscape(emailID) + "/context"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create email context request: %w", err)
	}
	req.Header.Set("x-api-key", c.APIKey)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get email context: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("get email context returned status %d", resp.StatusCode)
	}
	var out Context
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode email context: %w", err)
	}
	return &out, nil
}
