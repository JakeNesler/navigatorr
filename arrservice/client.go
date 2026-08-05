package arrservice

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// maxReadBytes caps how much of a response body is read into memory.
const maxReadBytes = 64 << 20 // 64MB

// Ping makes a lightweight authenticated request and reports whether the
// service answers and accepts the API key.
//
// The underlying error is unwrapped rather than formatted, because a *url.Error
// stringifies the full request URL — which carries the API key for services
// configured with query auth.
func (s *Service) Ping(ctx context.Context) string {
	_, code, err := s.DoRequest(ctx, "GET", s.StatusPath, nil, nil)
	if err != nil {
		var uerr *url.Error
		if errors.As(err, &uerr) && uerr.Err != nil {
			return "unreachable: " + uerr.Err.Error()
		}
		return "unreachable"
	}

	switch {
	case code >= 200 && code <= 299:
		return "ok"
	case code == 401 || code == 403:
		return "unauthorized — check api_key"
	default:
		return fmt.Sprintf("http %d", code)
	}
}

// DoRequest performs an authenticated HTTP request against a service.
func (s *Service) DoRequest(ctx context.Context, method, path string, query map[string]string, body []byte) ([]byte, int, error) {
	reqURL := s.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	// Apply auth
	s.Auth.Apply(req)

	// Apply query params
	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Hard ceiling on what we will hold in memory. The configurable
	// max_response_size_kb guard runs later and protects the model's context;
	// this protects the process itself, so it is deliberately far above any
	// legitimate *arr response rather than a second tuning knob.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxReadBytes+1))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response: %w", err)
	}
	if len(respBody) > maxReadBytes {
		return nil, resp.StatusCode, fmt.Errorf(
			"response from %s exceeds the %dMB read limit", s.Name, maxReadBytes>>20)
	}

	return respBody, resp.StatusCode, nil
}
