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

// Put stores data in the cache. The write goes to a temp file and is renamed
// into place, so an interrupted write cannot leave a truncated spec that the
// next 24 hours of cache hits would fail to parse.
func (c *Cache) Put(url string, data []byte) error {
	tmp, err := os.CreateTemp(c.dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.cacheFile(url))
}

// Invalidate removes a cached entry.
func (c *Cache) Invalidate(url string) {
	os.Remove(c.cacheFile(url))
}
