package qbit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/jakenesler/navigatorr/internal"
)

type httpStatusError struct {
	code int
	body string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.code, e.body)
}

// Client is a qBittorrent Web API client with cookie-based auth.
type Client struct {
	url      string
	username string
	password string
	http     *http.Client
	loggedIn bool
	mu       sync.Mutex
}

// NewClient creates a new qBittorrent client.
func NewClient(baseURL, username, password string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	jar, _ := cookiejar.New(nil)
	return &Client{
		url:      baseURL,
		username: username,
		password: password,
		http: &http.Client{
			Timeout:       30 * time.Second,
			Jar:           jar,
			CheckRedirect: internal.SameOriginRedirect,
		},
	}
}

// login authenticates with qBittorrent and stores the session cookie.
func (c *Client) login(ctx context.Context) error {
	form := url.Values{
		"username": {c.username},
		"password": {c.password},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.url+"/api/v2/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		return internal.HTTPError("creating login request", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return internal.HTTPError("login request", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != "Ok." {
		return fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	c.loggedIn = true
	return nil
}

// do executes an HTTP request, auto-logging in on 403.
func (c *Client) do(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for attempt := 0; attempt < 2; attempt++ {
		if !c.loggedIn {
			if err := c.login(ctx); err != nil {
				return nil, err
			}
		}

		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}

		req, err := http.NewRequestWithContext(ctx, method, c.url+path, body)
		if err != nil {
			return nil, internal.HTTPError("creating request", err)
		}
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		resp, err := c.http.Do(req)
		if err != nil {
			return nil, internal.HTTPError("executing request", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 403 {
			// Session expired, re-login
			c.loggedIn = false
			io.ReadAll(resp.Body)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode != 200 {
			return nil, &httpStatusError{code: resp.StatusCode, body: string(respBody)}
		}

		return respBody, nil
	}

	return nil, fmt.Errorf("failed after re-login retry")
}
