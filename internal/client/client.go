// Package client is a thin REST client for the BearMQ control-plane API.
// It has no dependency on terraform-plugin-framework so it can be unit-tested
// in isolation with net/http/httptest.
package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout   = 30 * time.Second
	maxRetries       = 4
	retryBaseDelay   = 250 * time.Millisecond
	userAgentDefault = "terraform-provider-bearmq"
)

// Config configures a Client.
type Config struct {
	// Endpoint is the base URL of the BearMQ instance, e.g. http://localhost:3333.
	Endpoint string
	// APIKey is sent as the X-API-KEY header. Exactly one of APIKey / Token is required.
	APIKey string
	// Token is sent as an Authorization: Bearer header.
	Token string
	// Insecure disables TLS certificate verification.
	Insecure bool
	// UserAgent overrides the default User-Agent header.
	UserAgent string
	// HTTPClient, when set, is used instead of a client built from the fields above.
	HTTPClient *http.Client
}

// Client talks to the BearMQ REST API.
type Client struct {
	endpoint  string
	apiKey    string
	token     string
	userAgent string
	http      *http.Client
}

// New validates cfg and returns a ready Client.
func New(cfg Config) (*Client, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(cfg.Endpoint), "/")
	if endpoint == "" {
		return nil, errors.New("bearmq: endpoint is required")
	}
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		return nil, fmt.Errorf("bearmq: endpoint %q must start with http:// or https://", endpoint)
	}
	if (cfg.APIKey == "") == (cfg.Token == "") {
		return nil, errors.New("bearmq: exactly one of api_key or token must be set")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if cfg.Insecure {
			transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in
		}
		httpClient = &http.Client{Timeout: defaultTimeout, Transport: transport}
	}

	ua := cfg.UserAgent
	if ua == "" {
		ua = userAgentDefault
	}

	return &Client{
		endpoint:  endpoint,
		apiKey:    cfg.APIKey,
		token:     cfg.Token,
		userAgent: ua,
		http:      httpClient,
	}, nil
}

// do performs a request against path (which must start with "/"), JSON-encoding
// body when non-nil and JSON-decoding a 2xx response into out when non-nil.
// Non-2xx responses become *APIError. 429 and 5xx are retried with backoff.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var encoded []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("bearmq: encoding request body: %w", err)
		}
		encoded = b
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}

		status, respBody, err := c.roundTrip(ctx, method, path, encoded)
		if err != nil {
			lastErr = err
			continue // transport error — retry
		}

		if status >= 200 && status < 300 {
			if out == nil || len(respBody) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("bearmq: decoding %s %s response: %w", method, path, err)
			}
			return nil
		}

		apiErr := newAPIError(status, respBody)
		if status == http.StatusTooManyRequests || status >= 500 {
			lastErr = apiErr
			continue
		}
		return apiErr
	}

	return fmt.Errorf("bearmq: %s %s failed after %d attempts: %w", method, path, maxRetries+1, lastErr)
}

func (c *Client) roundTrip(ctx context.Context, method, path string, encoded []byte) (int, []byte, error) {
	var reader io.Reader
	if encoded != nil {
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.endpoint+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("bearmq: building request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if encoded != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("X-API-KEY", c.apiKey)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("bearmq: reading response body: %w", err)
	}
	return resp.StatusCode, data, nil
}

func backoff(attempt int) time.Duration {
	d := retryBaseDelay
	for i := 1; i < attempt; i++ {
		d *= 2
	}
	return d
}

func decodeErrorBody(body []byte) apiErrorResponse {
	var parsed apiErrorResponse
	_ = json.Unmarshal(body, &parsed)
	return parsed
}
