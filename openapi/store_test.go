package openapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jakenesler/navigatorr/config"
)

const validTestSpec = `{"openapi":"3.0.0","info":{"title":"test","version":"1"},"paths":{"/thing":{"get":{"responses":{"200":{"description":"ok"}}}}}}`

func TestRefreshPreservesValidCacheOnFailure(t *testing.T) {
	for _, all := range []bool{false, true} {
		for _, invalid := range []bool{false, true} {
			var fail atomic.Bool
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if fail.Load() {
					if invalid {
						w.Write([]byte("<html>gateway error</html>"))
					} else {
						w.WriteHeader(http.StatusServiceUnavailable)
					}
					return
				}
				w.Write([]byte(validTestSpec))
			}))
			cfg := &config.Config{Services: map[string]config.ServiceConfig{"test": {OpenAPIURL: srv.URL}}}
			cache := NewCache(t.TempDir())
			store := &Store{cfg: cfg, cache: cache, indices: make(map[string]*Index)}
			store.LoadAll(context.Background())
			before := store.GetIndex("test")
			if before == nil {
				t.Fatal("initial spec did not load")
			}
			fail.Store(true)
			if all {
				if store.RefreshAll(context.Background())["test"] == nil {
					t.Fatal("refresh-all failure was hidden")
				}
			} else if err := store.Refresh(context.Background(), "test"); err == nil {
				t.Fatal("refresh failure was hidden")
			}
			if store.GetIndex("test") != before {
				t.Error("failed refresh replaced the in-memory index")
			}
			if string(cache.Get(srv.URL)) != validTestSpec {
				t.Error("failed refresh deleted or poisoned the disk cache")
			}
			restarted := &Store{cfg: cfg, cache: cache, indices: make(map[string]*Index)}
			restarted.LoadAll(context.Background())
			if restarted.GetIndex("test") == nil {
				t.Error("restart lost docs after a failed refresh")
			}
			srv.Close()
		}
	}
}

func TestLoadUsesStaleValidCacheWhenOffline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	cache := NewCache(t.TempDir())
	if err := cache.Put(srv.URL, []byte(validTestSpec)); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * cacheTTL)
	if err := os.Chtimes(cache.cacheFile(srv.URL), old, old); err != nil {
		t.Fatal(err)
	}
	store := &Store{cache: cache, indices: make(map[string]*Index)}
	if err := store.load(context.Background(), "test", srv.URL); err != nil {
		t.Fatalf("offline startup should use the valid stale spec: %v", err)
	}
	if store.GetIndex("test") == nil {
		t.Fatal("stale fallback did not populate the index")
	}
}

func TestLoadRecoversFromInvalidCache(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Write([]byte(validTestSpec))
	}))
	defer srv.Close()
	cache := NewCache(t.TempDir())
	if err := cache.Put(srv.URL, []byte("<html>cached gateway error</html>")); err != nil {
		t.Fatal(err)
	}
	store := &Store{cache: cache, indices: make(map[string]*Index)}
	if err := store.load(context.Background(), "test", srv.URL); err != nil {
		t.Fatalf("valid upstream spec should replace invalid cached data: %v", err)
	}
	if requests.Load() != 1 || string(cache.Get(srv.URL)) != validTestSpec {
		t.Fatal("invalid cache did not recover from upstream")
	}
}

func TestRefreshReplacesCacheAndIndexAfterSuccess(t *testing.T) {
	var updated atomic.Bool
	newSpec := strings.ReplaceAll(validTestSpec, "/thing", "/replacement")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if updated.Load() {
			w.Write([]byte(newSpec))
		} else {
			w.Write([]byte(validTestSpec))
		}
	}))
	defer srv.Close()
	cfg := &config.Config{Services: map[string]config.ServiceConfig{"test": {OpenAPIURL: srv.URL}}}
	cache := NewCache(t.TempDir())
	store := &Store{cfg: cfg, cache: cache, indices: make(map[string]*Index)}
	store.LoadAll(context.Background())
	updated.Store(true)
	if err := store.Refresh(context.Background(), "test"); err != nil {
		t.Fatal(err)
	}
	if string(cache.Get(srv.URL)) != newSpec {
		t.Fatal("successful refresh kept old cache data")
	}
	if _, err := store.GetIndex("test").GetDetail("/replacement", "GET"); err != nil {
		t.Fatal("successful refresh kept the old index")
	}
}

func TestInvalidDownloadIsNotCached(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>gateway error</html>"))
	}))
	defer srv.Close()
	cache := NewCache(t.TempDir())
	store := &Store{cache: cache, indices: make(map[string]*Index)}
	if err := store.load(context.Background(), "test", srv.URL); err == nil {
		t.Fatal("invalid download was accepted")
	}
	if cache.GetStale(srv.URL) != nil || store.GetIndex("test") != nil {
		t.Fatal("invalid download was published")
	}
}
