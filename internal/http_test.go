package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSameOriginRedirect(t *testing.T) {
	for _, tt := range []struct {
		from, to string
		allowed  bool
	}{
		{"https://example.com/start", "https://example.com/end", true},
		{"https://example.com/start", "https://EXAMPLE.com:443/end", true},
		{"http://example.com:80/start", "http://example.com/end", true},
		{"http://[::1]/start", "http://[::1]:80/end", true},
		{"https://example.com/start", "http://example.com/end", false},
		{"http://example.com/start", "https://example.com/end", false},
		{"https://example.com/start", "https://example.com:8443/end", false},
		{"https://example.com/start", "https://sub.example.com/end", false},
		{"https://example.com/start", "https://other.example/end", false},
	} {
		t.Run(tt.from+" to "+tt.to, func(t *testing.T) {
			first, _ := http.NewRequest("GET", tt.from, nil)
			next, _ := http.NewRequest("GET", tt.to, nil)
			err := SameOriginRedirect(next, []*http.Request{first})
			if (err == nil) != tt.allowed {
				t.Fatalf("redirect allowed = %v, want %v", err == nil, tt.allowed)
			}
		})
	}
}

func TestSameOriginRedirectPreservesAuthAndLimitsLoops(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/end", http.StatusTemporaryRedirect)
			return
		}
		if r.Header.Get("X-Api-Key") != "test-key" {
			t.Error("same-origin redirect lost authentication")
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	client := &http.Client{CheckRedirect: SameOriginRedirect}
	req, _ := http.NewRequest("GET", srv.URL+"/start", nil)
	req.Header.Set("X-Api-Key", "test-key")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Request.URL.Path != "/end" {
		t.Fatal("same-origin redirect was not followed")
	}
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = req
	}
	if err := SameOriginRedirect(req, via); err == nil {
		t.Fatal("redirect limit was removed")
	}
}

func TestHTTPErrorRedactsURLsAndPreservesCause(t *testing.T) {
	cause := &url.Error{Op: "Get", URL: "http://host/?apikey=test-key", Err: context.Canceled}
	err := HTTPError("executing request", fmt.Errorf("outer: %w", &url.Error{Op: "Get", URL: cause.URL, Err: cause}))
	if strings.Contains(err.Error(), "test-key") || strings.Contains(err.Error(), "http://") {
		t.Fatal("request URL survived redaction")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("redaction lost the cancellation cause")
	}
}
