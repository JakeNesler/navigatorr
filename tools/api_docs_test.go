package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/jakenesler/navigatorr/config"
	"github.com/jakenesler/navigatorr/openapi"
)

// list_services claims to report connection status, so it must actually probe.
func TestListServicesReportsStatus(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"version":"4.0"}`))
	}))
	defer ok.Close()

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer denied.Close()

	// Bound to a port nothing is listening on.
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	cfg := &config.Config{Services: map[string]config.ServiceConfig{
		"sonarr": {URL: ok.URL, APIKey: "k", AuthMethod: "header", AuthHeader: "X-Api-Key", APIVersion: "/api/v3"},
		"radarr": {URL: denied.URL, APIKey: "bad", AuthMethod: "header", AuthHeader: "X-Api-Key", APIVersion: "/api/v3"},
		"lidarr": {URL: deadURL, APIKey: "k", AuthMethod: "header", AuthHeader: "X-Api-Key", APIVersion: "/api/v1"},
	}}

	res, err := handleListServices(context.Background(), arrservice.NewRegistry(cfg), openapi.NewStore(cfg))
	if err != nil {
		t.Fatal(err)
	}

	var got []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(resultText(t, res)), &got); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, resultText(t, res))
	}

	want := map[string]string{
		"sonarr": "ok",
		"radarr": "unauthorized",
		"lidarr": "unreachable",
	}
	if len(got) != 3 {
		t.Fatalf("got %d services, want 3", len(got))
	}
	for _, g := range got {
		if !strings.HasPrefix(g.Status, want[g.Name]) {
			t.Errorf("%s status = %q, want prefix %q", g.Name, g.Status, want[g.Name])
		}
	}
}

// A url.Error stringifies the full request URL, which carries the key for
// query-auth services. The unreachable path must not echo it.
func TestPingDoesNotLeakQueryAuthKey(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	const secret = "super-secret-key"
	svc := arrservice.NewService("custom", config.ServiceConfig{
		URL: deadURL, APIKey: secret, AuthMethod: "query", APIVersion: "/api",
	})

	status := svc.Ping(context.Background())
	if !strings.HasPrefix(status, "unreachable") {
		t.Fatalf("status = %q, want unreachable", status)
	}
	if strings.Contains(status, secret) {
		t.Fatalf("Ping leaked the API key into its status: %q", status)
	}
}
