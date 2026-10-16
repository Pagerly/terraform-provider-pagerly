// Package client is a small HTTP client for the Pagerly v1 REST API.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the production API host.
const DefaultBaseURL = "https://api.pagerly.io"

// Client talks to /api/v1 with an API key.
//
// Writes are serialised: Terraform applies independent resources in parallel, and
// Pagerly stores most configuration on one organisation record that each write
// reads, changes and saves whole. Two overlapping writes would have the later save
// drop the earlier one's change, so only one write is in flight at a time. Reads
// still run in parallel.
type Client struct {
	BaseURL   string
	APIKey    string
	UserAgent string
	HTTP      *http.Client

	writes sync.Mutex
}

// APIError is a non-2xx reply, with the message the API put in its body.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Pagerly API returned %d: %s", e.Status, e.Message)
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Status == http.StatusNotFound
}

// New returns a client for baseURL (DefaultBaseURL when empty).
func New(baseURL, apiKey, userAgent string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		APIKey:    apiKey,
		UserAgent: userAgent,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
	}
}

// Do sends one request. body is JSON-encoded when non-nil; out is decoded from
// the reply when non-nil and the reply has a body.
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	if method != http.MethodGet {
		c.writes.Lock()
		defer c.writes.Unlock()
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(b)
	}
	u := c.BaseURL + "/api/v1" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-APIKEY", c.APIKey)
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &APIError{Status: resp.StatusCode, Message: errorMessage(data, resp.Status)}
	}
	if out != nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding reply from %s %s: %w", method, path, err)
		}
	}
	return nil
}

func errorMessage(data []byte, fallback string) string {
	var parsed struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &parsed) == nil {
		if parsed.Error != "" {
			return parsed.Error
		}
		if parsed.Message != "" {
			return parsed.Message
		}
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return fallback
	}
	if len(text) > 300 {
		text = text[:300] + "…"
	}
	return text
}

// PathEscape escapes one path segment (an email, a group name).
func PathEscape(segment string) string {
	return url.PathEscape(segment)
}

// Me is GET /me.
type Me struct {
	OrgID         string `json:"org_id"`
	WorkspaceName string `json:"workspace_name"`
	KeyName       string `json:"key_name"`
	ActsAs        string `json:"acts_as"`
	OrgAdmin      bool   `json:"org_admin"`
}

// Me checks the key and returns who it acts as.
func (c *Client) Me(ctx context.Context) (*Me, error) {
	var me Me
	if err := c.Do(ctx, http.MethodGet, "/me", nil, nil, &me); err != nil {
		return nil, err
	}
	return &me, nil
}
