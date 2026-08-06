package sabnzbd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stub returns a client pointed at a test server, plus a pointer to the query
// string of the last request it received.
func stub(t *testing.T, body string, status int) (*Client, *string) {
	t.Helper()
	var lastQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastQuery = r.URL.RawQuery
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "", "testkey"), &lastQuery
}

func TestNewClientURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		urlBase string
		want    string
	}{
		{"default url_base", "http://host:8080", "/sabnzbd", "http://host:8080/sabnzbd/api"},
		{"empty url_base", "http://host:8080", "", "http://host:8080/api"},
		{"trailing slash on url", "http://host:8080/", "/sabnzbd", "http://host:8080/sabnzbd/api"},
		{"url_base without slashes", "http://host:8080", "sabnzbd", "http://host:8080/sabnzbd/api"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewClient(tt.baseURL, tt.urlBase, "k").url; got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// SABnzbd reports a rejected call with HTTP 200 and status:false, so the status
// code alone would let a failure through as a successful result.
func TestDoRejectsErrorEnvelope(t *testing.T) {
	client, _ := stub(t, `{"status":false,"error":"API Key Required"}`, 200)

	_, err := client.Do(context.Background(), "queue", nil)
	if err == nil {
		t.Fatal("expected an error for status:false, got nil")
	}
	if !strings.Contains(err.Error(), "API Key Required") {
		t.Errorf("error should carry SABnzbd's message, got: %v", err)
	}
}

// status:true is the success shape for the action modes and must not be treated
// as a failure.
func TestDoAcceptsStatusTrue(t *testing.T) {
	client, _ := stub(t, `{"status":true,"nzo_ids":["SABnzbd_nzo_abc123"]}`, 200)

	if _, err := client.Do(context.Background(), "addurl", nil); err != nil {
		t.Fatalf("status:true should succeed, got: %v", err)
	}
}

func TestDoSendsModeAndKeyAndDropsEmptyParams(t *testing.T) {
	client, query := stub(t, `{"queue":{}}`, 200)

	_, err := client.Do(context.Background(), "queue", map[string]string{
		"limit":  "5",
		"cat":    "",
		"search": "",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{"mode=queue", "apikey=testkey", "output=json", "limit=5"} {
		if !strings.Contains(*query, want) {
			t.Errorf("query %q missing %q", *query, want)
		}
	}
	for _, unwanted := range []string{"cat=", "search="} {
		if strings.Contains(*query, unwanted) {
			t.Errorf("empty param should have been dropped, query was %q", *query)
		}
	}
}

func TestGetHistoryUnwrapsSlots(t *testing.T) {
	body := `{"history":{"noofslots":2,"slots":[
		{"nzo_id":"a","name":"first","status":"Completed"},
		{"nzo_id":"b","name":"second","status":"Failed","fail_message":"bad par2"}
	]}}`
	client, query := stub(t, body, 200)

	history, err := client.GetHistory(context.Background(), 0, 2, "", "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history.Slots) != 2 {
		t.Fatalf("got %d slots, want 2", len(history.Slots))
	}
	if history.Slots[1].FailMessage != "bad par2" {
		t.Errorf("fail_message did not decode, got %q", history.Slots[1].FailMessage)
	}
	if !strings.Contains(*query, "failed_only=1") {
		t.Errorf("failed_only was not sent, query was %q", *query)
	}
}

// The API key travels in the query string, and a *url.Error stringifies the URL
// it was built from. These errors go straight into tool output, so the key must
// not survive into the message on either the transport or the request-building
// path.
func TestDoErrorsDoNotCarryAPIKey(t *testing.T) {
	const key = "SUPER-SECRET-APIKEY"

	// A closed listener gives a deterministic connection failure without
	// depending on a port being free.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	tests := []struct {
		name    string
		baseURL string
	}{
		{"transport failure", closedURL},
		{"request build failure", "http://127.0.0.1:\x7f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.baseURL, "", key)

			_, err := client.Do(context.Background(), "queue", nil)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if strings.Contains(err.Error(), key) {
				t.Errorf("API key leaked into error: %v", err)
			}
		})
	}
}

func TestQueueActionSendsNameAndValue(t *testing.T) {
	client, query := stub(t, `{"status":true}`, 200)

	if _, err := client.QueueAction(context.Background(), "priority", "SABnzbd_nzo_abc", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, want := range []string{"mode=queue", "name=priority", "value=SABnzbd_nzo_abc", "value2=1"} {
		if !strings.Contains(*query, want) {
			t.Errorf("query %q missing %q", *query, want)
		}
	}
}
