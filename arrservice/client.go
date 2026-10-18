package arrservice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jakenesler/navigatorr/internal"
)

var httpClient = &http.Client{
	Timeout:       30 * time.Second,
	CheckRedirect: internal.SameOriginRedirect,
}

// maxPingRedirects mirrors the cap Go's http.Client applies when CheckRedirect
// is nil.
const maxPingRedirects = 10

// pingClient follows a redirect only when it points back at the same resource,
// which is the trailing-slash bounce SABnzbd does on /sabnzbd. A redirect that
// goes somewhere else is a service sending an unauthenticated or misrouted
// request to its login or setup page, and following that answers 200 and makes
// Ping report a service nobody can call as ok.
var pingClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		// Setting CheckRedirect replaces Go's default cap of 10, so it has to
		// be reimposed here. A server that alternates /x and /x/ looks like the
		// same resource on every hop, and without a cap Ping follows it as fast
		// as the network allows until the status timeout fires.
		if len(via) >= maxPingRedirects {
			return http.ErrUseLastResponse
		}
		if len(via) == 0 || internal.SameOriginRedirect(req, via) != nil || !sameResource(via[0].URL, req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	},
}

// sameResource reports whether two URLs address the same host and path, ignoring
// a trailing slash.
func sameResource(a, b *url.URL) bool {
	return a.Host == b.Host &&
		strings.TrimSuffix(a.Path, "/") == strings.TrimSuffix(b.Path, "/")
}

// maxReadBytes caps how much of a response body is read into memory.
const maxReadBytes = 64 << 20 // 64MB

// Ping makes a lightweight authenticated request and reports whether the
// service answers and accepts the API key.
func (s *Service) Ping(ctx context.Context) string {
	_, code, err := s.doRequest(ctx, pingClient, "GET", s.StatusPath, nil, nil)
	if err != nil {
		return "unreachable: " + err.Error()
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
	return s.doRequest(ctx, httpClient, method, path, query, body)
}

func (s *Service) doRequest(ctx context.Context, client *http.Client, method, path string, query map[string]string, body []byte) ([]byte, int, error) {
	reqURL := s.BaseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, 0, internal.HTTPError("creating request", err)
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

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, internal.HTTPError("executing request", err)
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
