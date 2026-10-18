package qbit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v2/auth/login" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "test-session", Path: "/"})
			w.Write([]byte("Ok."))
			return
		}
		if _, err := r.Cookie("SID"); err != nil {
			t.Error("request is missing the login cookie")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL, "test-user", "test-password")
}

func TestAddTorrentChecksResult(t *testing.T) {
	for _, body := range []string{"Ok.", "Fails.", " Fails.\n"} {
		t.Run(strings.TrimSpace(body), func(t *testing.T) {
			client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/v2/torrents/add" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				r.ParseForm()
				if r.Form.Get("urls") != "magnet:?xt=test" || r.Form.Get("savepath") != "/downloads" {
					t.Error("add parameters were lost")
				}
				w.Write([]byte(body))
			})
			err := client.AddTorrent(context.Background(), "magnet:?xt=test", "/downloads")
			if (err != nil) != (strings.TrimSpace(body) == "Fails.") {
				t.Fatalf("AddTorrent response %q: error = %v", body, err)
			}
		})
	}
}

func TestTorrentStateActionsSupportModernAndLegacyServers(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, action := range []string{"pause", "resume"} {
			name := action + "/modern"
			if legacy {
				name = action + "/legacy"
			}
			t.Run(name, func(t *testing.T) {
				modern := "stop"
				if action == "resume" {
					modern = "start"
				}
				var paths []string
				client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
					paths = append(paths, r.URL.Path)
					r.ParseForm()
					if r.Method != "POST" || r.Form.Get("hashes") != "a|b" {
						t.Error("torrent action lost method or hashes")
					}
					if legacy && r.URL.Path == "/api/v2/torrents/"+modern {
						w.WriteHeader(http.StatusNotFound)
					}
				})
				var err error
				if action == "pause" {
					err = client.PauseTorrents(context.Background(), []string{"a", "b"})
				} else {
					err = client.ResumeTorrents(context.Background(), []string{"a", "b"})
				}
				if err != nil {
					t.Fatal(err)
				}
				want := []string{"/api/v2/torrents/" + modern}
				if legacy {
					want = append(want, "/api/v2/torrents/"+action)
				}
				if !reflect.DeepEqual(paths, want) {
					t.Fatalf("requests = %v, want %v", paths, want)
				}
			})
		}
	}
}

func TestTorrentStateDoesNotRetryServerFailure(t *testing.T) {
	var calls int
	client := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	})
	if err := client.PauseTorrents(context.Background(), []string{"a"}); err == nil {
		t.Fatal("server failure was reported as success")
	}
	if calls != 1 {
		t.Fatalf("server failure was retried %d times", calls-1)
	}
}
