package openapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

var fetchClient = &http.Client{Timeout: 30 * time.Second}

// Fetch downloads a spec. The store parses it before updating the cache.
func Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/json, application/x-yaml, text/yaml")

	resp, err := fetchClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching spec from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching spec from %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}

	return data, nil
}
