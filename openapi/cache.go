package openapi

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const cacheTTL = 24 * time.Hour

// Cache handles disk caching of OpenAPI specs.
type Cache struct {
	dir string
}

// NewCache creates a cache in the given directory.
func NewCache(dir string) *Cache {
	os.MkdirAll(dir, 0755)
	return &Cache{dir: dir}
}

func (c *Cache) cacheFile(url string) string {
	hash := sha256.Sum256([]byte(url))
	return filepath.Join(c.dir, fmt.Sprintf("%x.json", hash[:8]))
}

// Get returns cached data if fresh, or nil if stale/missing.
func (c *Cache) Get(url string) []byte {
	path := c.cacheFile(url)
	info, err := os.Stat(path)
	if err != nil {
		return nil
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return data
}

// Put stores data in the cache.
func (c *Cache) Put(url string, data []byte) error {
	return os.WriteFile(c.cacheFile(url), data, 0644)
}

// Invalidate removes a cached entry.
func (c *Cache) Invalidate(url string) {
	os.Remove(c.cacheFile(url))
}
