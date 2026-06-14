package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the analysis configuration.
type Config struct {
	// Patterns are the package patterns to analyze.
	Patterns []string `json:"patterns"`
	// Dir is the working directory (default: current directory).
	Dir string `json:"dir,omitempty"`
	// CacheDir is the cache directory (default: ".rivus-cache").
	CacheDir string `json:"cache_dir,omitempty"`
	// Format is the output format: "json" or "table".
	Format string `json:"format,omitempty"`
	// Output is the output file path (empty = stdout).
	Output string `json:"output,omitempty"`
	// NoCache disables the cache.
	NoCache bool `json:"no_cache,omitempty"`
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Patterns: []string{"./..."},
		CacheDir: ".rivus-cache",
		Format:   "table",
	}
}

// LoadFile loads a config from a JSON file.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SaveFile saves a config to a JSON file.
func SaveFile(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Merge applies non-zero values from other to cfg.
func (cfg *Config) Merge(other *Config) {
	if len(other.Patterns) > 0 {
		cfg.Patterns = other.Patterns
	}
	if other.Dir != "" {
		cfg.Dir = other.Dir
	}
	if other.CacheDir != "" {
		cfg.CacheDir = other.CacheDir
	}
	if other.Format != "" {
		cfg.Format = other.Format
	}
	if other.Output != "" {
		cfg.Output = other.Output
	}
	if other.NoCache {
		cfg.NoCache = true
	}
}
