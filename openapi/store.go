package openapi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	data, err := Fetch(ctx, url, s.cache)
	if err != nil {
		return err
	}

	idx, err := Parse(ctx, name, data)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.indices[name] = idx
	s.mu.Unlock()

	return nil
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

	var results []EndpointSummary
	for name, idx := range s.indices {
		if serviceName != "" && name != serviceName {
			continue
		}
		results = append(results, idx.Search(query)...)
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

	// Invalidate cache
	s.cache.Invalidate(svc.OpenAPIURL)

	return s.load(ctx, name, svc.OpenAPIURL)
}

// RefreshAll re-fetches specs for all services.
func (s *Store) RefreshAll(ctx context.Context) map[string]error {
	errors := make(map[string]error)
	for name, svc := range s.cfg.Services {
		if svc.OpenAPIURL == "" {
			continue
		}
		s.cache.Invalidate(svc.OpenAPIURL)
		if err := s.load(ctx, name, svc.OpenAPIURL); err != nil {
			errors[name] = err
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}
