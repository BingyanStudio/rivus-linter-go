package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheRoundtrip(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	// Store some data.
	data := json.RawMessage(`[{"name":"Foo","flags":[]}]`)
	if err := c.Store("example.com/pkg", "abc123", data); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Retrieve with matching hash.
	got, ok := c.Get("example.com/pkg", "abc123")
	if !ok {
		t.Fatal("Get returned false for matching hash")
	}
	// Compare structurally since cache may re-format JSON.
	var gotParsed, wantParsed interface{}
	json.Unmarshal(got, &gotParsed)
	json.Unmarshal(data, &wantParsed)
	gotJSON, _ := json.Marshal(gotParsed)
	wantJSON, _ := json.Marshal(wantParsed)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("Get returned %s, want %s", got, data)
	}

	// Retrieve with different hash.
	_, ok = c.Get("example.com/pkg", "different")
	if ok {
		t.Error("Get returned true for non-matching hash")
	}
}

func TestCacheClear(t *testing.T) {
	dir := t.TempDir()
	c := NewCache(dir)

	data := json.RawMessage(`[]`)
	c.Store("pkg", "hash", data)

	if err := c.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	_, ok := c.Get("pkg", "hash")
	if ok {
		t.Error("expected cache miss after clear")
	}
}

func TestPackageHash(t *testing.T) {
	dir := t.TempDir()

	// Write a Go file.
	goFile := filepath.Join(dir, "test.go")
	os.WriteFile(goFile, []byte("package test\n"), 0o644)

	hash1, err := PackageHash(dir)
	if err != nil {
		t.Fatalf("PackageHash failed: %v", err)
	}

	// Same content should produce same hash.
	hash2, err := PackageHash(dir)
	if err != nil {
		t.Fatalf("PackageHash failed: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("expected same hash, got %s and %s", hash1, hash2)
	}

	// Different content should produce different hash.
	os.WriteFile(goFile, []byte("package test\n// changed\n"), 0o644)
	hash3, err := PackageHash(dir)
	if err != nil {
		t.Fatalf("PackageHash failed: %v", err)
	}
	if hash1 == hash3 {
		t.Error("expected different hash after file change")
	}
}
