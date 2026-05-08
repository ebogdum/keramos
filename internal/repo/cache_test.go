package repo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestCacheKey(t *testing.T) {
	key1 := cacheKey("https://example.com/repo")
	key2 := cacheKey("https://example.com/repo")
	key3 := cacheKey("https://other.com/repo")

	if key1 != key2 {
		t.Errorf("same URL should produce same key: %s != %s", key1, key2)
	}

	if key1 == key3 {
		t.Error("different URLs should produce different keys")
	}

	if 64 != len(key1) {
		t.Errorf("expected 64-char hex SHA256, got %d chars", len(key1))
	}
}

func TestCachePutAndGet(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"mypkg": {
				{Name: "mypkg", Version: "1.0.0", Digest: "abc123"},
			},
		},
		Generated: time.Now(),
	}

	repoURL := "https://example.com/repo"
	err := cache.Put(repoURL, idx, "etag-value")
	if nil != err {
		t.Fatalf("Put failed: %v", err)
	}

	got, meta, ok := cache.Get(repoURL)
	if !ok {
		t.Fatal("expected cache hit")
	}

	if nil == got {
		t.Fatal("expected non-nil index")
	}

	if nil == meta {
		t.Fatal("expected non-nil meta")
	}

	if "etag-value" != meta.ETag {
		t.Errorf("expected etag 'etag-value', got %q", meta.ETag)
	}

	if repoURL != meta.URL {
		t.Errorf("expected URL %q, got %q", repoURL, meta.URL)
	}

	entries, ok := got.Entries["mypkg"]
	if !ok {
		t.Fatal("expected mypkg in cached entries")
	}

	if 1 != len(entries) {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if "1.0.0" != entries[0].Version {
		t.Errorf("expected version 1.0.0, got %s", entries[0].Version)
	}
}

func TestCacheTTLExpiry(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      1 * time.Millisecond,
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries:    map[string][]IndexEntry{},
	}

	repoURL := "https://example.com/repo"
	err := cache.Put(repoURL, idx, "")
	if nil != err {
		t.Fatalf("Put failed: %v", err)
	}

	// Force expiry by backdating the meta
	key := cacheKey(repoURL)
	metaPath := filepath.Join(cache.CacheDir, key, "meta.json")
	meta := cacheMeta{
		FetchedAt: time.Now().Add(-1 * time.Hour),
		URL:       repoURL,
	}
	metaData, _ := json.Marshal(&meta)
	if err := os.WriteFile(metaPath, metaData, 0644); nil != err {
		t.Fatalf("failed to backdate meta: %v", err)
	}

	got, _, ok := cache.Get(repoURL)
	if ok {
		t.Error("expected cache miss after TTL expiry")
	}
	if nil != got {
		t.Error("expected nil index on cache miss")
	}
}

func TestCacheGetMiss(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	got, meta, ok := cache.Get("https://nonexistent.com/repo")
	if ok {
		t.Error("expected cache miss for non-existent entry")
	}
	if nil != got {
		t.Error("expected nil index")
	}
	if nil != meta {
		t.Error("expected nil meta")
	}
}

func TestCacheInvalidate(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries:    map[string][]IndexEntry{},
	}

	repoURL := "https://example.com/repo"
	err := cache.Put(repoURL, idx, "")
	if nil != err {
		t.Fatalf("Put failed: %v", err)
	}

	err = cache.Invalidate(repoURL)
	if nil != err {
		t.Fatalf("Invalidate failed: %v", err)
	}

	_, _, ok := cache.Get(repoURL)
	if ok {
		t.Error("expected cache miss after invalidation")
	}
}

func TestCacheInvalidateAll(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries:    map[string][]IndexEntry{},
	}

	err := cache.Put("https://a.com", idx, "")
	if nil != err {
		t.Fatalf("Put a failed: %v", err)
	}

	err = cache.Put("https://b.com", idx, "")
	if nil != err {
		t.Fatalf("Put b failed: %v", err)
	}

	err = cache.InvalidateAll()
	if nil != err {
		t.Fatalf("InvalidateAll failed: %v", err)
	}

	entries, err := os.ReadDir(cache.CacheDir)
	if nil != err {
		t.Fatalf("failed to read cache dir: %v", err)
	}

	if 0 != len(entries) {
		t.Errorf("expected empty cache dir, got %d entries", len(entries))
	}
}

func TestCacheMetaRoundTrip(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries: map[string][]IndexEntry{
			"pkg": {{Name: "pkg", Version: "2.0.0", Digest: "def456"}},
		},
	}

	repoURL := "https://example.com/packages"
	etag := `"abc123-etag"`
	err := cache.Put(repoURL, idx, etag)
	if nil != err {
		t.Fatalf("Put failed: %v", err)
	}

	// Read raw meta file and verify structure
	key := cacheKey(repoURL)
	metaPath := filepath.Join(cache.CacheDir, key, "meta.json")
	raw, err := os.ReadFile(metaPath)
	if nil != err {
		t.Fatalf("failed to read meta file: %v", err)
	}

	var meta cacheMeta
	if err := json.Unmarshal(raw, &meta); nil != err {
		t.Fatalf("failed to unmarshal meta: %v", err)
	}

	if etag != meta.ETag {
		t.Errorf("expected etag %q, got %q", etag, meta.ETag)
	}

	if repoURL != meta.URL {
		t.Errorf("expected url %q, got %q", repoURL, meta.URL)
	}

	// Read raw index file and verify structure
	indexPath := filepath.Join(cache.CacheDir, key, "index.yaml")
	indexRaw, err := os.ReadFile(indexPath)
	if nil != err {
		t.Fatalf("failed to read index file: %v", err)
	}

	var readIdx IndexFile
	if err := yaml.Unmarshal(indexRaw, &readIdx); nil != err {
		t.Fatalf("failed to unmarshal index: %v", err)
	}

	if "v1" != readIdx.APIVersion {
		t.Errorf("expected apiVersion v1, got %s", readIdx.APIVersion)
	}
}

func TestCacheInvalidateNonExistent(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	err := cache.Invalidate("https://nonexistent.example.com")
	if nil != err {
		t.Errorf("Invalidate of non-existent entry should not error: %v", err)
	}
}

func TestCacheInvalidateAllEmptyDir(t *testing.T) {
	cache := &IndexCache{
		CacheDir: t.TempDir(),
		TTL:      30 * time.Minute,
	}

	err := cache.InvalidateAll()
	if nil != err {
		t.Errorf("InvalidateAll on empty dir should not error: %v", err)
	}
}
