// Package client is a thin HTTP wrapper around the StatusPulse REST API.
// It speaks the same JSON shapes as api/models.go and uses the X-API-Key
// header for authentication (the API accepts session tokens there too).
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is the StatusPulse API client. Zero value is not usable — use New.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New returns a client pointed at baseURL, authenticating with token.
// An empty token is allowed — login endpoints work unauthenticated.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// WithBasicAuth wires HTTP Basic Auth onto every outgoing request. Used when
// the deploy is sitting behind the temporary `SITE_PASSWORD` gate — the MCP
// + CLI need to clear that gate before the API's own X-API-Key auth kicks in.
// Empty user OR password is a no-op.
func (c *Client) WithBasicAuth(user, pass string) *Client {
	if user == "" || pass == "" {
		return c
	}
	base := c.http.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	c.http.Transport = &basicAuthTransport{base: base, user: user, pass: pass}
	return c
}

type basicAuthTransport struct {
	base       http.RoundTripper
	user, pass string
}

func (t *basicAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r2 := r.Clone(r.Context())
	r2.SetBasicAuth(t.user, t.pass)
	return t.base.RoundTrip(r2)
}

// APIError is returned when the API responds with a non-2xx status. The
// Message, Code, and Plan fields are populated from the standard error body
// shape emitted by the API: `{"error": "...", "code": "...", "plan": "..."}`.
// Code lets the CLI tell "monitor limit reached" apart from a generic 403;
// Plan tells the user which tier they're on so the upgrade hint is accurate.
type APIError struct {
	Status  int
	Message string
	Code    string
	Plan    string
}

func (e *APIError) Error() string {
	// The human-readable message is the API's own `error` field — no
	// transport prefix. The CLI's handleAPIError adds context on top.
	if e.Message == "" {
		return fmt.Sprintf("request failed (HTTP %d)", e.Status)
	}
	return e.Message
}

// IsUnauthorized reports whether err is an APIError with status 401.
func IsUnauthorized(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusUnauthorized
}

// do performs a request against the API and decodes the response into out
// when non-nil. A nil body is sent if body is nil.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("X-API-Key", c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		var apiErr struct {
			Error string `json:"error"`
			Code  string `json:"code"`
			Plan  string `json:"plan"`
		}
		_ = json.Unmarshal(respBody, &apiErr)
		return &APIError{
			Status:  resp.StatusCode,
			Message: apiErr.Error,
			Code:    apiErr.Code,
			Plan:    apiErr.Plan,
		}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
