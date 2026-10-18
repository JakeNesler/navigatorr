package internal_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jakenesler/navigatorr/arrservice"
	"github.com/jakenesler/navigatorr/config"
	"github.com/jakenesler/navigatorr/qbit"
	"github.com/jakenesler/navigatorr/sabnzbd"
	"github.com/jakenesler/navigatorr/transmission"
)

func TestAuthenticatedClientsRejectCrossOriginRedirects(t *testing.T) {
	clients := []struct {
		name string
		call func(string) error
	}{
		{"arr header", func(base string) error {
			s := arrservice.NewService("custom", config.ServiceConfig{URL: base, AuthMethod: "header", APIKey: "test-key"})
			_, _, err := s.DoRequest(context.Background(), "GET", "/status", nil, nil)
			return err
		}},
		{"arr query", func(base string) error {
			s := arrservice.NewService("custom", config.ServiceConfig{URL: base, AuthMethod: "query", APIKey: "test-key"})
			_, _, err := s.DoRequest(context.Background(), "GET", "/status", nil, nil)
			return err
		}},
		{"arr basic", func(base string) error {
			s := arrservice.NewService("custom", config.ServiceConfig{URL: base, AuthMethod: "basic", APIKey: "test-user"})
			_, _, err := s.DoRequest(context.Background(), "GET", "/status", nil, nil)
			return err
		}},
		{"sabnzbd", func(base string) error {
			_, err := sabnzbd.NewClient(base, "", "test-key").Do(context.Background(), "queue", nil)
			return err
		}},
		{"qbittorrent", func(base string) error {
			_, err := qbit.NewClient(base, "test-user", "test-password").GetTransferInfo(context.Background())
			return err
		}},
		{"transmission", func(base string) error {
			_, err := transmission.NewClient(base, "test-user", "test-password").SessionStats(context.Background())
			return err
		}},
	}

	for _, client := range clients {
		for _, status := range []int{http.StatusFound, http.StatusTemporaryRedirect} {
			for _, crossHost := range []bool{false, true} {
				name := client.name + "/" + http.StatusText(status)
				if crossHost {
					name += "/host"
				} else {
					name += "/port"
				}
				t.Run(name, func(t *testing.T) {
					var reached atomic.Int32
					sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						reached.Add(1)
						w.Write([]byte(`{}`))
					}))
					defer sink.Close()
					target := sink.URL
					if crossHost {
						target = strings.Replace(target, "127.0.0.1", "localhost", 1)
					}
					source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						if client.name == "transmission" && r.Header.Get("X-Transmission-Session-Id") == "" {
							w.Header().Set("X-Transmission-Session-Id", "test-session")
							w.WriteHeader(http.StatusConflict)
							return
						}
						http.Redirect(w, r, target, status)
					}))
					defer source.Close()

					if err := client.call(source.URL); err == nil {
						t.Error("cross-origin redirect should return an error")
					}
					if reached.Load() != 0 {
						t.Fatal("authenticated client contacted the redirected origin")
					}
				})
			}
		}
	}
}
