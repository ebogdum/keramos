package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/logger"
	"gopkg.in/yaml.v3"
)

// IndexCache provides on-disk caching for repository index files.
type IndexCache struct {
	CacheDir string
	TTL      time.Duration
}

type cacheMeta struct {
	FetchedAt time.Time `json:"fetchedAt"`
	ETag      string    `json:"etag,omitempty"`
	URL       string    `json:"url"`
}

// NewIndexCache creates an IndexCache using ~/.cache/keramos/indexes/ with a 30-minute TTL.
func NewIndexCache() (*IndexCache, error) {
	cacheDir, err := os.UserCacheDir()
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to determine cache directory", err)
	}

	dir := filepath.Join(cacheDir, "keramos", "indexes")
	if err := os.MkdirAll(dir, 0755); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to create cache directory", err)
	}

	return &IndexCache{
		CacheDir: dir,
		TTL:      30 * time.Minute,
	}, nil
}

// Get returns a cached index if it exists and is within the TTL window.
func (c *IndexCache) Get(repoURL string) (*IndexFile, *cacheMeta, bool) {
	key := cacheKey(repoURL)
	entryDir := filepath.Join(c.CacheDir, key)

	metaPath := filepath.Join(entryDir, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if nil != err {
		return nil, nil, false
	}

	var meta cacheMeta
	if err := json.Unmarshal(metaData, &meta); nil != err {
		logger.Debug("corrupt cache meta for %s, ignoring", repoURL)
		return nil, nil, false
	}

	if time.Since(meta.FetchedAt) > c.TTL {
		logger.Debug("cache expired for %s", repoURL)
		return nil, &meta, false
	}

	indexPath := filepath.Join(entryDir, "index.yaml")
	indexData, err := os.ReadFile(indexPath)
	if nil != err {
		return nil, &meta, false
	}

	var idx IndexFile
	if err := yaml.Unmarshal(indexData, &idx); nil != err {
		logger.Debug("corrupt cached index for %s, ignoring", repoURL)
		return nil, &meta, false
	}

	if nil == idx.Entries {
		idx.Entries = make(map[string][]IndexEntry)
	}

	logger.Debug("cache hit for %s", repoURL)
	return &idx, &meta, true
}

// Put stores an index and its metadata in the cache.
func (c *IndexCache) Put(repoURL string, idx *IndexFile, etag string) error {
	key := cacheKey(repoURL)
	entryDir := filepath.Join(c.CacheDir, key)

	if err := os.MkdirAll(entryDir, 0755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to create cache entry directory", err)
	}

	indexData, err := yaml.Marshal(idx)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to marshal index for cache", err)
	}

	indexPath := filepath.Join(entryDir, "index.yaml")
	if err := os.WriteFile(indexPath, indexData, 0644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write cached index", err)
	}

	meta := cacheMeta{
		FetchedAt: time.Now(),
		ETag:      etag,
		URL:       repoURL,
	}

	metaData, err := json.Marshal(&meta)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to marshal cache metadata", err)
	}

	metaPath := filepath.Join(entryDir, "meta.json")
	if err := os.WriteFile(metaPath, metaData, 0644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write cache metadata", err)
	}

	logger.Debug("cached index for %s", repoURL)
	return nil
}

// Invalidate removes the cache entry for a specific repository URL.
func (c *IndexCache) Invalidate(repoURL string) error {
	key := cacheKey(repoURL)
	entryDir := filepath.Join(c.CacheDir, key)

	if err := os.RemoveAll(entryDir); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to invalidate cache entry", err)
	}

	logger.Debug("invalidated cache for %s", repoURL)
	return nil
}

// InvalidateAll removes all cached indexes.
func (c *IndexCache) InvalidateAll() error {
	entries, err := os.ReadDir(c.CacheDir)
	if nil != err {
		if os.IsNotExist(err) {
			return nil
		}
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to read cache directory", err)
	}

	for _, entry := range entries {
		entryPath := filepath.Join(c.CacheDir, entry.Name())
		if err := os.RemoveAll(entryPath); nil != err {
			return keramoserr.WrapError(keramoserr.ErrRepo, "failed to clear cache entry", err)
		}
	}

	logger.Debug("invalidated all cached indexes")
	return nil
}

// cacheKey produces a SHA256 hex string from a repository URL for use as a directory name.
func cacheKey(repoURL string) string {
	h := sha256.Sum256([]byte(repoURL))
	return hex.EncodeToString(h[:])
}
