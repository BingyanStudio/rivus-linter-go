package analyzer

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Cache provides file-hash-based persistent caching of analysis results.
type Cache struct {
	dir string
}

// NewCache creates a new Cache. If dir is empty, caching is disabled.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir}
}

// cacheIndex is the structure of the cache index file.
type cacheIndex struct {
	Version  string                  `json:"version"`
	Packages map[string]packageCache `json:"packages"`
}

// packageCache stores the hash and cached function results for a package.
type packageCache struct {
	FileHash  string          `json:"file_hash"`
	Functions json.RawMessage `json:"functions"`
	Timestamp time.Time       `json:"timestamp"`
}

// PackageHash computes a hash of all Go files in a directory.
func PackageHash(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	h := sha256.New()
	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".go" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		path := filepath.Join(dir, name)
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		io.Copy(h, f)
		f.Close()
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Get retrieves cached results for a package if the hash matches.
func (c *Cache) Get(pkgPath string, currentHash string) (json.RawMessage, bool) {
	if c.dir == "" {
		return nil, false
	}

	idx, err := c.loadIndex()
	if err != nil {
		return nil, false
	}

	pkg, ok := idx.Packages[pkgPath]
	if !ok {
		return nil, false
	}

	if pkg.FileHash != currentHash {
		return nil, false
	}

	return pkg.Functions, true
}

// Store saves results for a package in the cache.
func (c *Cache) Store(pkgPath string, fileHash string, functions json.RawMessage) error {
	if c.dir == "" {
		return nil
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return err
	}

	idx, err := c.loadIndex()
	if err != nil {
		idx = &cacheIndex{
			Version:  "1.0",
			Packages: make(map[string]packageCache),
		}
	}

	idx.Packages[pkgPath] = packageCache{
		FileHash:  fileHash,
		Functions: functions,
		Timestamp: time.Now(),
	}

	return c.saveIndex(idx)
}

// Clear removes the cache directory.
func (c *Cache) Clear() error {
	if c.dir == "" {
		return nil
	}
	return os.RemoveAll(c.dir)
}

func (c *Cache) indexPath() string {
	return filepath.Join(c.dir, "index.json")
}

func (c *Cache) loadIndex() (*cacheIndex, error) {
	data, err := os.ReadFile(c.indexPath())
	if err != nil {
		return nil, err
	}
	var idx cacheIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

func (c *Cache) saveIndex(idx *cacheIndex) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.indexPath(), data, 0o644)
}
