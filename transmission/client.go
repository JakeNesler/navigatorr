package transmission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const csrfHeader = "X-Transmission-Session-Id"

// Client is a Transmission RPC client with CSRF token handling.
type Client struct {
	url       string
	username  string
	password  string
	csrfToken string
	mu        sync.Mutex
	http      *http.Client
}

// NewClient creates a new Transmission RPC client.
func NewClient(url, username, password string) *Client {
	// Ensure URL ends with /transmission/rpc
	if url != "" && url[len(url)-1] != '/' {
		url += "/"
	}
	url += "transmission/rpc"

	return &Client{
		url:      url,
		username: username,
		password: password,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// call makes an RPC request, handling CSRF token refresh.
func (c *Client) call(ctx context.Context, method string, args any) (*rpcResponse, error) {
	reqBody := rpcRequest{
		Method:    method,
		Arguments: args,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	// Try the request, retry once if CSRF token is stale
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", c.url, bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		c.mu.Lock()
		if c.csrfToken != "" {
			req.Header.Set(csrfHeader, c.csrfToken)
		}
		c.mu.Unlock()

		if c.username != "" {
			req.SetBasicAuth(c.username, c.password)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, fmt.Errorf("executing request: %w", err)
		}

		// 409 = need new CSRF token
		if resp.StatusCode == 409 {
			token := resp.Header.Get(csrfHeader)
			resp.Body.Close()
			if token == "" {
				return nil, fmt.Errorf("got 409 but no CSRF token in response")
			}
			c.mu.Lock()
			c.csrfToken = token
			c.mu.Unlock()
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		var rpcResp rpcResponse
		if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}

		if rpcResp.Result != "success" {
			return nil, fmt.Errorf("RPC error: %s", rpcResp.Result)
		}

		return &rpcResp, nil
	}

	return nil, fmt.Errorf("failed after CSRF retry")
}
