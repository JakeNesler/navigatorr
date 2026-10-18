package openapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/jakenesler/navigatorr/config"
	"github.com/jakenesler/navigatorr/internal"
)

// Store manages OpenAPI specs for all services.
type Store struct {
	cfg     *config.Config
	cache   *Cache
	indices map[string]*Index
	mu      sync.RWMutex
}

// NewStore creates a new spec store.
func NewStore(cfg *config.Config) *Store {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".cache", "navigatorr")

	return &Store{
		cfg:     cfg,
		cache:   NewCache(cacheDir),
		indices: make(map[string]*Index),
	}
}

// LoadAll fetches and parses specs for all configured services.
func (s *Store) LoadAll(ctx context.Context) {
	for name, svc := range s.cfg.Services {
		if svc.OpenAPIURL == "" {
			continue
		}
		if err := s.load(ctx, name, svc.OpenAPIURL); err != nil {
			internal.Errorf("loading spec for %s: %v", name, err)
		} else {
			idx := s.GetIndex(name)
			if idx != nil {
				internal.Logf("loaded %s: %d endpoints", name, idx.Count())
			}
		}
	}
}

func (s *Store) load(ctx context.Context, name, url string) error {
	return s.loadSpec(ctx, name, url, false)
}

func (s *Store) loadSpec(ctx context.Context, name, url string, refresh bool) error {
	if !refresh {
		if data := s.cache.Get(url); data != nil {
			if idx, err := Parse(ctx, name, data); err == nil {
				s.setIndex(name, idx)
				return nil
			}
		}
	}

	data, err := Fetch(ctx, url)
	var idx *Index
	if err == nil {
		idx, err = Parse(ctx, name, data)
	}
	if err != nil {
		// Explicit refresh reports failure while keeping the existing cache
		// and index. Startup can use a valid stale spec to keep docs available.
		if !refresh && ctx.Err() == nil {
			if cached := s.cache.GetStale(url); cached != nil {
				if fallback, parseErr := Parse(ctx, name, cached); parseErr == nil {
					internal.Logf("using cached spec for %s after fetch failure: %v", name, err)
					s.setIndex(name, fallback)
					return nil
				}
			}
		}
		return err
	}

	if err := s.cache.Put(url, data); err != nil {
		internal.Errorf("caching spec for %s: %v", name, err)
	}
	s.setIndex(name, idx)
	return nil
}

func (s *Store) setIndex(name string, idx *Index) {
	s.mu.Lock()
	s.indices[name] = idx
	s.mu.Unlock()
}

// GetIndex returns the index for a service.
func (s *Store) GetIndex(name string) *Index {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.indices[name]
}

// Search searches across all or a specific service.
func (s *Store) Search(query, serviceName string) []EndpointSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.indices))
	for name := range s.indices {
		names = append(names, name)
	}
	sort.Strings(names)

	var results []EndpointSummary
	for _, name := range names {
		if serviceName != "" && name != serviceName {
			continue
		}
		results = append(results, s.indices[name].Search(query)...)
	}
	return results
}

// Refresh re-fetches the spec for a specific service.
func (s *Store) Refresh(ctx context.Context, name string) error {
	svc, ok := s.cfg.Services[name]
	if !ok {
		return fmt.Errorf("service %q not configured", name)
	}
	if svc.OpenAPIURL == "" {
		return fmt.Errorf("no OpenAPI URL for %s", name)
	}

	return s.loadSpec(ctx, name, svc.OpenAPIURL, true)
}

// RefreshAll re-fetches specs for all services.
func (s *Store) RefreshAll(ctx context.Context) map[string]error {
	errors := make(map[string]error)
	for name, svc := range s.cfg.Services {
		if svc.OpenAPIURL == "" {
			continue
		}
		if err := s.loadSpec(ctx, name, svc.OpenAPIURL, true); err != nil {
			errors[name] = err
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}
