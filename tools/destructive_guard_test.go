package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jakenesler/navigatorr/qbit"
	"github.com/jakenesler/navigatorr/transmission"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// callTool drives a registered tool by name, the way the MCP server would.
func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("tool %q was not registered", name)
	}

	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}}
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("%s returned a transport error: %v", name, err)
	}
	return res
}

// Deletes in both torrent clients travel as POSTs rather than as the DELETE
// verb, so the guard in call_api never sees them. The tools have to refuse on
// their own, and they have to refuse before a request goes out — a guard that
// only suppresses the response has already deleted the torrent.
func TestTorrentDeletesRespectAllowDestructive(t *testing.T) {
	tests := []struct {
		name   string
		tool   string
		args   map[string]any
		reject string
	}{
		{"qbit delete", "qbit_manage_torrent", map[string]any{"action": "delete", "hashes": "abc"}, "Deleting is disabled"},
		{"qbit delete_files", "qbit_manage_torrent", map[string]any{"action": "delete_files", "hashes": "abc"}, "Deleting is disabled"},
		{"transmission remove", "transmission_manage_torrent", map[string]any{"action": "remove", "ids": "1"}, "Removing is disabled"},
		{"transmission remove_data", "transmission_manage_torrent", map[string]any{"action": "remove_data", "ids": "1"}, "Removing is disabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var hits int
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits++
				w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)

			s := server.NewMCPServer("test", "0.0.0")
			registerQbitTools(s, qbit.NewClient(srv.URL, "u", "p"), false)
			registerTransmissionTools(s, transmission.NewClient(srv.URL, "u", "p"), false)

			res := callTool(t, s, tt.tool, tt.args)

			if !strings.Contains(resultText(t, res), tt.reject) {
				t.Errorf("expected a refusal mentioning %q, got: %s", tt.reject, resultText(t, res))
			}
			if hits != 0 {
				t.Errorf("guard let %d request(s) reach the server; it must refuse before any request goes out", hits)
			}
		})
	}
}

// The guard must not touch the non-destructive actions.
func TestNonDestructiveActionsAreNotGated(t *testing.T) {
	for _, tt := range []struct {
		name string
		tool string
		args map[string]any
	}{
		{"qbit pause", "qbit_manage_torrent", map[string]any{"action": "pause", "hashes": "abc"}},
		{"transmission stop", "transmission_manage_torrent", map[string]any{"action": "stop", "ids": "1"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s := server.NewMCPServer("test", "0.0.0")
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)

			registerQbitTools(s, qbit.NewClient(srv.URL, "u", "p"), false)
			registerTransmissionTools(s, transmission.NewClient(srv.URL, "u", "p"), false)

			text := resultText(t, callTool(t, s, tt.tool, tt.args))
			for _, refusal := range []string{"Deleting is disabled", "Removing is disabled"} {
				if strings.Contains(text, refusal) {
					t.Errorf("%s was gated but should not be: %s", tt.args["action"], text)
				}
			}
		})
	}
}

// With the setting on, the destructive actions reach the client.
func TestDeletesGoThroughWhenAllowed(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	s := server.NewMCPServer("test", "0.0.0")
	registerQbitTools(s, qbit.NewClient(srv.URL, "u", "p"), true)

	text := resultText(t, callTool(t, s, "qbit_manage_torrent", map[string]any{"action": "delete", "hashes": "abc"}))
	if strings.Contains(text, "Deleting is disabled") {
		t.Fatalf("delete was refused with allow_destructive on: %s", text)
	}
	if hits == 0 {
		t.Error("delete was allowed but no request reached the server")
	}
}
