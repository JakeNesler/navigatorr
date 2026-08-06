package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jakenesler/navigatorr/sabnzbd"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// manageItem drives the registered sabnzbd_manage_item handler and returns the
// result along with the query string SABnzbd would have received.
func manageItem(t *testing.T, allowDestructive bool, args map[string]any) (string, string) {
	t.Helper()

	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.Write([]byte(`{"status":true}`))
	}))
	t.Cleanup(srv.Close)

	s := server.NewMCPServer("test", "0.0.0")
	registerSabnzbdTools(s, sabnzbd.NewClient(srv.URL, "", "k"), allowDestructive)

	tool := s.GetTool("sabnzbd_manage_item")
	if tool == nil {
		t.Fatal("sabnzbd_manage_item was not registered")
	}

	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{Name: "sabnzbd_manage_item", Arguments: args},
	})
	if err != nil {
		t.Fatalf("handler returned a transport error: %v", err)
	}
	return resultText(t, res), lastQuery
}

// Queue and history are separate lists in SABnzbd, and delete against the queue
// does not touch a finished job. Without a target the history entries a user
// most wants to clear are unreachable.
func TestManageItemDeleteRoutesByTarget(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]any
		wantMode string
		wantFile bool
	}{
		{"queue is the default", map[string]any{"action": "delete", "nzo_id": "abc"}, "mode=queue", false},
		{"explicit queue", map[string]any{"action": "delete", "nzo_id": "abc", "target": "queue"}, "mode=queue", false},
		{"history", map[string]any{"action": "delete", "nzo_id": "abc", "target": "history"}, "mode=history", false},
		{"history with files", map[string]any{"action": "delete_files", "nzo_id": "abc", "target": "history"}, "mode=history", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, query := manageItem(t, true, tt.args)

			if !strings.Contains(query, tt.wantMode) {
				t.Errorf("query %q missing %q", query, tt.wantMode)
			}
			if !strings.Contains(query, "name=delete") {
				t.Errorf("query %q missing name=delete", query)
			}
			if got := strings.Contains(query, "del_files=1"); got != tt.wantFile {
				t.Errorf("del_files=1 present = %v, want %v (query %q)", got, tt.wantFile, query)
			}
		})
	}
}

// A bad target must not fall through to the queue, which would delete the wrong
// thing while reporting success.
func TestManageItemRejectsBadTarget(t *testing.T) {
	text, query := manageItem(t, true, map[string]any{"action": "delete", "nzo_id": "abc", "target": "hisotry"})

	if !strings.Contains(text, "unknown target") {
		t.Errorf("expected a refusal, got: %s", text)
	}
	if query != "" {
		t.Errorf("a request went out despite the bad target: %s", query)
	}
}

// target=history is only meaningful for delete. Pairing it with pause must not
// quietly pause the queue job that happens to share the id.
func TestManageItemRejectsHistoryTargetOnNonDelete(t *testing.T) {
	text, query := manageItem(t, true, map[string]any{"action": "pause", "nzo_id": "abc", "target": "history"})

	if !strings.Contains(text, "only applies to delete") {
		t.Errorf("expected a refusal, got: %s", text)
	}
	if query != "" {
		t.Errorf("a request went out despite the invalid combination: %s", query)
	}
}

// The allow_destructive gate has to cover history deletion too, or the setting
// is bypassable by passing a target.
func TestManageItemHistoryDeleteIsGated(t *testing.T) {
	text, query := manageItem(t, false, map[string]any{"action": "delete", "nzo_id": "abc", "target": "history"})

	if !strings.Contains(text, "Deleting is disabled") {
		t.Errorf("expected the destructive guard to refuse, got: %s", text)
	}
	if query != "" {
		t.Errorf("a delete reached SABnzbd with allow_destructive off: %s", query)
	}
}
