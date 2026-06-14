package analyzer

import "os"

// Cache provides file-hash-based persistent caching of analysis results.
type Cache struct {
	dir string
}

// NewCache creates a new Cache. If dir is empty, caching is disabled.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir}
}

// Clear removes the cache directory.
func (c *Cache) Clear() error {
	if c.dir == "" {
		return nil
	}
	return os.RemoveAll(c.dir)
}
