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
	"github.com/mark3labs/mcp-go/mcp"
)

// callAPI drives handleCallAPI against a stub *arr service.
func callAPI(t *testing.T, handler http.HandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := &config.Config{
		Services: map[string]config.ServiceConfig{
			"sonarr": {URL: srv.URL, APIKey: "k", AuthMethod: "header", AuthHeader: "X-Api-Key", APIVersion: "/api/v3"},
		},
	}
	registry := arrservice.NewRegistry(cfg)

	if _, ok := args["service"]; !ok {
		args["service"] = "sonarr"
	}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: "call_api", Arguments: args}}

	res, err := handleCallAPI(context.Background(), req, registry, 50, false)
	if err != nil {
		t.Fatalf("handleCallAPI returned a transport error: %v", err)
	}
	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	var sb strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// A non-2xx response carrying a JSON body must not read as success. *arr
// services return JSON on failure, so this previously parsed cleanly and the
// caller could not tell an auth failure from a real result.
func TestCallAPIStatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{"unauthorized with json body", 401, `{"message":"Unauthorized"}`, true},
		{"unprocessable with json body", 422, `[{"errorMessage":"bad title"}]`, true},
		{"server error with json body", 500, `{"message":"boom"}`, true},
		{"not found with empty body", 404, ``, true},
		{"ok", 200, `{"id":1}`, false},
		{"created", 201, `{"id":2}`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := callAPI(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}, map[string]any{"path": "/series"})

			if res.IsError != tt.wantErr {
				t.Fatalf("status %d: IsError = %v, want %v (body: %s)",
					tt.status, res.IsError, tt.wantErr, resultText(t, res))
			}
			if tt.wantErr && !strings.Contains(resultText(t, res), "HTTP") {
				t.Errorf("error result should name the status, got: %s", resultText(t, res))
			}
		})
	}
}

// The API key must reach the service, and a lowercase method must still be
// recognized by the DELETE guard.
func TestCallAPIAuthAndDeleteGuard(t *testing.T) {
	var gotKey, gotMethod string
	h := func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		gotMethod = r.Method
		w.Write([]byte(`{"ok":true}`))
	}

	res := callAPI(t, h, map[string]any{"path": "/series"})
	if res.IsError {
		t.Fatalf("unexpected error: %s", resultText(t, res))
	}
	if gotKey != "k" {
		t.Errorf("X-Api-Key = %q, want %q", gotKey, "k")
	}
	if gotMethod != "GET" {
		t.Errorf("method = %q, want GET", gotMethod)
	}

	for _, m := range []string{"DELETE", "delete", " delete "} {
		res := callAPI(t, h, map[string]any{"path": "/series/1", "method": m})
		if !res.IsError {
			t.Errorf("method %q: DELETE guard did not block it", m)
			continue
		}
		// Must be stopped by the guard, not by the HTTP layer rejecting a
		// malformed method — otherwise the guard is failing open.
		if !strings.Contains(resultText(t, res), "allow_destructive") {
			t.Errorf("method %q: blocked by something other than the guard: %s", m, resultText(t, res))
		}
	}
}

func TestApplyFilter(t *testing.T) {
	items := []any{
		map[string]any{"title": "Alpha", "year": float64(2001), "hasFile": true},
		map[string]any{"title": "Beta", "year": float64(2020), "hasFile": false},
		map[string]any{"title": "Gamma", "year": float64(2015)}, // hasFile absent
	}

	tests := []struct {
		name   string
		filter string
		want   []string
	}{
		{"contains is case-insensitive", "title:contains:alp", []string{"Alpha"}},
		{"gt compares numerically", "year:gt:2010", []string{"Beta", "Gamma"}},
		{"lt compares numerically", "year:lt:2010", []string{"Alpha"}},
		{"eq matches", "hasFile:eq:true", []string{"Alpha"}},
		{"ne includes items missing the field", "hasFile:ne:true", []string{"Beta", "Gamma"}},
		{"no matches yields empty, not null", "title:contains:zzz", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilter(items, tt.filter)
			if got == nil {
				t.Fatal("applyFilter returned nil; it must marshal as [] not null")
			}
			var titles []string
			for _, it := range got {
				titles = append(titles, it.(map[string]any)["title"].(string))
			}
			if len(titles) != len(tt.want) {
				t.Fatalf("got %v, want %v", titles, tt.want)
			}
			for i := range titles {
				if titles[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", titles, tt.want)
				}
			}

			data, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) == "null" {
				t.Error("empty filter result marshaled to null")
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 100); got != "short" {
		t.Errorf("truncate should leave short strings alone, got %q", got)
	}
	long := strings.Repeat("x", 300)
	got := truncate(long, 100)
	if len(got) <= 100 || !strings.Contains(got, "truncated") {
		t.Errorf("truncate(300 chars, 100) = %d chars, want a truncation marker", len(got))
	}
	if !strings.HasPrefix(got, strings.Repeat("x", 100)) {
		t.Error("truncate should keep the first max bytes")
	}
}

// Some APIs wrap their list one level deeper than the *arr convention, e.g.
// SABnzbd returns {"history": {"slots": [...]}} rather than {"records": [...]}.
// Field selection used to return an empty object for those: the drilling check
// only looked for an array directly under the requested key, and pickFields had
// no branch for arrays, so "history.slots.name" was dropped without an error.
func TestCallAPINestedArrayFields(t *testing.T) {
	body := `{"history":{"noofslots":2,"slots":[
		{"name":"first release","size":"1.2 GB","fail_message":""},
		{"name":"second release","size":"3.4 GB","fail_message":"bad par2"}
	]}}`

	res := callAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}, map[string]any{"path": "/api", "fields": "history.slots.name"})

	got := resultText(t, res)
	for _, want := range []string{"first release", "second release"} {
		if !strings.Contains(got, want) {
			t.Errorf("field selection dropped %q from a two-level response:\n%s", want, got)
		}
	}
	if strings.Contains(got, "3.4 GB") {
		t.Errorf("field selection returned unrequested fields:\n%s", got)
	}
}

// The one-level *arr shape has to keep working unchanged.
func TestCallAPISingleLevelArrayFieldsStillWork(t *testing.T) {
	body := `{"page":1,"records":[{"title":"a","year":2001},{"title":"b","year":2002}]}`

	res := callAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}, map[string]any{"path": "/wanted/missing", "fields": "records.title"})

	got := resultText(t, res)
	if !strings.Contains(got, `"a"`) || !strings.Contains(got, `"b"`) {
		t.Errorf("records.title stopped working:\n%s", got)
	}
	if strings.Contains(got, "2001") {
		t.Errorf("records.title returned unrequested fields:\n%s", got)
	}
}
