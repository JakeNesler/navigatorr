package arrservice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jakenesler/navigatorr/config"
)

// A slow endpoint is the case request_timeout_seconds exists for, so a service
// built through the registry has to actually carry the configured value rather
// than falling back to the package default.
func TestRegistryAppliesConfiguredRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1300 * time.Millisecond)
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	svcCfg := config.ServiceConfig{URL: srv.URL, APIVersion: "/api/v1", APIKey: "k"}

	// 1s against a response that takes longer. The package default of 30s would
	// let this through, so a pass here means the configured value was the one
	// used.
	t.Run("a timeout below the response time cuts it off", func(t *testing.T) {
		reg := NewRegistry(&config.Config{
			Services:              map[string]config.ServiceConfig{"sonarr": svcCfg},
			RequestTimeoutSeconds: 1,
		})
		svc, _ := reg.Get("sonarr")

		if _, _, err := svc.DoRequest(context.Background(), "GET", "/series", nil, nil); err == nil {
			t.Fatal("expected a timeout, got nil")
		}
	})

	t.Run("a timeout above it lets the call through", func(t *testing.T) {
		reg := NewRegistry(&config.Config{
			Services:              map[string]config.ServiceConfig{"sonarr": svcCfg},
			RequestTimeoutSeconds: 10,
		})
		svc, _ := reg.Get("sonarr")

		_, code, err := svc.DoRequest(context.Background(), "GET", "/series", nil, nil)
		if err != nil {
			t.Fatalf("DoRequest: %v", err)
		}
		if code != http.StatusOK {
			t.Errorf("code = %d, want 200", code)
		}
	})
}

// Ping keeps its own budget. Sharing the request timeout would make a raised
// value turn list_services against a down host into a multi-minute wait, and a
// lowered one cut off status checks that were fine.
func TestPingDoesNotUseTheRequestTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1300 * time.Millisecond)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	reg := NewRegistry(&config.Config{
		Services:              map[string]config.ServiceConfig{"sonarr": {URL: srv.URL, APIVersion: "/api/v1", APIKey: "k"}},
		RequestTimeoutSeconds: 1,
	})
	svc, _ := reg.Get("sonarr")
	svc.StatusPath = "/status"

	if _, _, err := svc.DoRequest(context.Background(), "GET", "/series", nil, nil); err == nil {
		t.Fatal("call_api should have hit the 1s request timeout")
	}
	// Same server, same delay, through the client Ping uses.
	if got := svc.Ping(context.Background()); got != "ok" {
		t.Errorf("Ping = %q, want ok: it followed request_timeout_seconds instead of its own budget", got)
	}
}

// A Config built in Go rather than loaded from YAML leaves the timeout at zero,
// which http.Client reads as no timeout at all.
func TestRegistryDefaultsAZeroRequestTimeout(t *testing.T) {
	reg := NewRegistry(&config.Config{
		Services: map[string]config.ServiceConfig{"sonarr": {URL: "http://127.0.0.1:9", APIVersion: "/api/v1"}},
	})
	svc, _ := reg.Get("sonarr")

	want := time.Duration(config.DefaultRequestTimeoutSeconds) * time.Second
	if svc.client.Timeout != want {
		t.Errorf("client timeout = %v, want the %v default rather than an unbounded client", svc.client.Timeout, want)
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
