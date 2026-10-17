package arrservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jakenesler/navigatorr/config"
)

// A *url.Error stringifies the URL it was built from, and call_api formats
// these errors straight into tool output. The transport reports the URL it
// built, which for auth_method: query carries the API key, and url.Parse
// reports the raw one, which can carry a credential the config put there.
func TestDoRequestErrorsDoNotCarryCredentials(t *testing.T) {
	// A closed listener gives a deterministic connection failure without
	// depending on a port being free.
	closed := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := closed.URL
	closed.Close()

	tests := []struct {
		name   string
		cfg    config.ServiceConfig
		path   string
		secret string
	}{
		{
			name:   "transport failure carries the applied api key",
			cfg:    config.ServiceConfig{URL: closedURL, APIVersion: "/sabnzbd", AuthMethod: "query", APIKey: "SECRET-APIKEY"},
			path:   "/api",
			secret: "SECRET-APIKEY",
		},
		{
			// The path comes from the model, so it decides whether url.Parse
			// fails at all.
			name:   "unparseable url carries credentials from the config",
			cfg:    config.ServiceConfig{URL: "http://admin:SECRET-PASSWORD@127.0.0.1:9", AuthMethod: "query", APIKey: "k"},
			path:   "/api\x7f",
			secret: "SECRET-PASSWORD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService("sabnzbd", tt.cfg)

			_, _, err := svc.DoRequest(context.Background(), "GET", tt.path, nil, nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if strings.Contains(err.Error(), tt.secret) {
				t.Errorf("credential leaked into error: %v", err)
			}
		})
	}
}

// A service that redirects an unauthenticated API request to its own login or
// setup page must not report ok, which is what following the redirect produces.
func TestPingDoesNotFollowRedirectToLoginPage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
	})
	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>sign in</html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := NewService("prowlarr", config.ServiceConfig{
		URL:        srv.URL,
		APIVersion: "/api/v1",
		APIKey:     "wrong",
	})
	svc.StatusPath = "/status"

	if got := svc.Ping(context.Background()); got == "ok" {
		t.Errorf("Ping reported ok for a service that redirected to %s", "/auth/login")
	}
}

func TestPingReportsUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	svc := NewService("prowlarr", config.ServiceConfig{
		URL:        srv.URL,
		APIVersion: "/api/v1",
		APIKey:     "wrong",
	})

	if got := svc.Ping(context.Background()); got != "unauthorized — check api_key" {
		t.Errorf("Ping = %q, want the unauthorized message", got)
	}
}

// A redirect that stays on the service's own API is the service working, not a
// bounce to a login page. SABnzbd sends /sabnzbd to /sabnzbd/ on every request.
func TestPingFollowsRedirectWithinTheAPI(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sabnzbd", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/sabnzbd/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/sabnzbd/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := NewService("sabnzbd", config.ServiceConfig{
		URL:        srv.URL,
		APIVersion: "/sabnzbd",
		APIKey:     "key",
	})

	if got := svc.Ping(context.Background()); got != "ok" {
		t.Errorf("Ping = %q, want ok for a redirect that stays on the API", got)
	}
}

// DoRequest still follows redirects, so a service behind a proxy that upgrades
// or rewrites the request keeps working through call_api.
func TestDoRequestStillFollowsRedirects(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/series", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/moved", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/api/v1/moved", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	svc := NewService("sonarr", config.ServiceConfig{
		URL:        srv.URL,
		APIVersion: "/api/v1",
		APIKey:     "key",
	})

	_, code, err := svc.DoRequest(context.Background(), "GET", "/series", nil, nil)
	if err != nil {
		t.Fatalf("DoRequest: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("code = %d, want 200", code)
	}
}

// Setting CheckRedirect opts out of the 10-redirect cap net/http applies by
// default, so a server that alternates /x and /x/ — which sameResource treats
// as one resource on every hop — is followed as fast as the network allows
// until the status timeout fires. That turns a status check into thousands of
// requests against an already-misconfigured service.
func TestPingStopsFollowingRedirectLoop(t *testing.T) {
	var hits int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		if r.URL.Path == "/api/v1/status" {
			http.Redirect(w, r, "/api/v1/status/", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/api/v1/status", http.StatusFound)
	}))
	defer srv.Close()

	svc := NewService("prowlarr", config.ServiceConfig{
		URL:        srv.URL,
		APIVersion: "/api/v1",
		APIKey:     "k",
	})
	svc.StatusPath = "/status"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	got := svc.Ping(ctx)

	if n := atomic.LoadInt64(&hits); n > maxPingRedirects+1 {
		t.Errorf("Ping made %d requests, want at most %d", n, maxPingRedirects+1)
	}
	if got != "http 302" {
		t.Errorf("Ping() = %q, want %q", got, "http 302")
	}
}
