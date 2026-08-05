package sabnzbd

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

// maxReadBytes caps how much of a response body is read into memory.
const maxReadBytes = 64 << 20 // 64MB

// Client is a SABnzbd API client. SABnzbd dispatches every call from a "mode"
// query parameter against a single endpoint instead of using separate paths.
type Client struct {
	url    string
	apiKey string
	http   *http.Client
}

// NewClient creates a new SABnzbd client. urlBase is SABnzbd's own url_base
// setting, which ships as /sabnzbd but is often cleared.
func NewClient(baseURL, urlBase, apiKey string) *Client {
	base := strings.TrimRight(baseURL, "/")
	if trimmed := strings.Trim(urlBase, "/"); trimmed != "" {
		base += "/" + trimmed
	}
	return &Client{
		url:    base + "/api",
		apiKey: apiKey,
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

// errorEnvelope is SABnzbd's failure shape. It arrives with HTTP 200, so a
// status code check on its own reports a rejected call as a success.
type errorEnvelope struct {
	Status *bool  `json:"status"`
	Error  string `json:"error"`
}

// unwrapURLError strips the URL from a *url.Error. SABnzbd auth rides in the
// query string, so the stringified URL carries the API key, and these errors
// are formatted straight into tool output.
func unwrapURLError(err error, stage string) error {
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		return fmt.Errorf("%s: %w", stage, uerr.Err)
	}
	return fmt.Errorf("%s: %w", stage, err)
}

// Do calls a single SABnzbd mode and returns the raw JSON body. Empty params
// are dropped so callers can pass optional values through unconditionally.
func (c *Client) Do(ctx context.Context, mode string, params map[string]string) ([]byte, error) {
	q := url.Values{}
	q.Set("mode", mode)
	q.Set("output", "json")
	q.Set("apikey", c.apiKey)
	for k, v := range params {
		if v != "" {
			q.Set(k, v)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.url+"?"+q.Encode(), nil)
	if err != nil {
		return nil, unwrapURLError(err, "creating request")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, unwrapURLError(err, "executing request")
	}
	defer resp.Body.Close()

	// Hard ceiling on what we will hold in memory, matching arrservice. History
	// responses are the ones that get big.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if len(body) > maxReadBytes {
		return nil, fmt.Errorf("response from sabnzbd exceeds the %dMB read limit", maxReadBytes>>20)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}

	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Status != nil && !*env.Status && env.Error != "" {
		return nil, fmt.Errorf("sabnzbd rejected mode=%s: %s", mode, env.Error)
	}

	return body, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
