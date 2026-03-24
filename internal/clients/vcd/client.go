// Package vcd provides a client for the VMware Cloud Director API.
// It handles authentication, retries, and JSON/XML codec automatically.
//
// The client uses lazy authentication — it acquires a bearer token on
// the first request and caches it. If a request returns 401, the client
// re-authenticates once and retries.
package vcd

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/CloudKey-io/hbs-queue/internal/retry"
)

// Client is a VCD API client with lazy authentication and automatic retries.
type Client struct {
	baseURL    string
	version    string
	username   string
	password   string
	org        string
	httpClient *http.Client
	logger     *slog.Logger

	mu    sync.Mutex
	token string
}

// New creates a VCD client. The client does not authenticate until the
// first API call is made.
func New(baseURL, version, username, password, org string, logger *slog.Logger) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		version:    version,
		username:   username,
		password:   password,
		org:        org,
		httpClient: newHTTPClient(),
		logger:     logger,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

// authenticate acquires a bearer token from the VCD provider session endpoint.
func (c *Client) authenticate(ctx context.Context) error {
	path := "/cloudapi/1.0.0/sessions/provider"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("create auth request: %w", err)
	}

	req.Header.Set("Accept", fmt.Sprintf("application/*;version=%s", c.version))
	req.SetBasicAuth(c.username+"@"+c.org, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return &APIError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Method:     http.MethodPost,
			Path:       path,
		}
	}

	token := resp.Header.Get("X-VMWARE-VCLOUD-ACCESS-TOKEN")
	if token == "" {
		return fmt.Errorf("auth response missing X-VMWARE-VCLOUD-ACCESS-TOKEN header")
	}

	c.token = token
	c.logger.Debug("vcd auth token acquired")
	return nil
}

// getToken returns the cached token, authenticating first if needed.
func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" {
		return c.token, nil
	}
	if err := c.authenticate(ctx); err != nil {
		return "", err
	}
	return c.token, nil
}

// clearToken removes the cached token so the next call re-authenticates.
func (c *Client) clearToken() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
}

// do executes an HTTP request against the VCD API. It handles authentication,
// retries on transient errors, and decodes the JSON response into result.
//
// If body is nil, no request body is sent. If result is nil, the response
// body is discarded.
func (c *Client) do(ctx context.Context, method, path string, body io.Reader, result any) error {
	var lastErr error

	err := retry.Do(ctx, 3, func() error {
		token, err := c.getToken(ctx)
		if err != nil {
			return fmt.Errorf("get token: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		req.Header.Set("Accept", fmt.Sprintf("application/*;version=%s", c.version))
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("%s %s: %w", method, path, err)
		}
		defer resp.Body.Close() //nolint:errcheck // best-effort close on read path

		// Re-authenticate on 401 and signal retry.
		if resp.StatusCode == http.StatusUnauthorized {
			c.clearToken()
			c.logger.Debug("vcd token expired, re-authenticating")
			lastErr = &APIError{StatusCode: 401, Method: method, Path: path}
			return lastErr
		}

		// Non-2xx: return APIError.
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			respBody, _ := io.ReadAll(resp.Body)
			apiErr := &APIError{
				StatusCode: resp.StatusCode,
				Body:       string(respBody),
				Method:     method,
				Path:       path,
			}
			if apiErr.IsRetryable() {
				lastErr = apiErr
				return apiErr
			}
			// Non-retryable: don't retry, return immediately.
			return apiErr
		}

		// Success: decode response.
		if result != nil {
			if err := decodeJSON(resp.Body, result); err != nil {
				return fmt.Errorf("decode %s %s: %w", method, path, err)
			}
		}

		return nil
	})

	return err
}
