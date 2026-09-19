package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/pkg"
	"gopkg.in/yaml.v3"
)

// IndexFile represents a repository index (index.yaml).
type IndexFile struct {
	APIVersion string                  `yaml:"apiVersion"`
	Entries    map[string][]IndexEntry `yaml:"entries"`
	Generated  time.Time               `yaml:"generated"`
}

// IndexEntry represents a single package version in the index.
type IndexEntry struct {
	Name        string      `yaml:"name"`
	Version     string      `yaml:"version"`
	AppVersion  string      `yaml:"appVersion,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Digest      string      `yaml:"digest"`
	URLs        []string    `yaml:"urls"`
	Created     time.Time   `yaml:"created"`
	Provenance  string      `yaml:"provenance,omitempty"`
	Signatures  []Signature `yaml:"signatures,omitempty"`
}

// Signature represents a cryptographic signature for a package.
type Signature struct {
	Type   string `yaml:"type"`
	Digest string `yaml:"digest"`
	URL    string `yaml:"url,omitempty"`
}

// GenerateIndex creates an index.yaml from .keramos.tgz files in a directory.
func GenerateIndex(dir, baseURL string) (*IndexFile, error) {
	absDir, err := filepath.Abs(dir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to resolve directory path", err)
	}

	entries, err := os.ReadDir(absDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read directory", err)
	}

	idx := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
		Generated:  time.Now(),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".keramos.tgz") {
			continue
		}

		archivePath := filepath.Join(absDir, entry.Name())
		indexEntry, err := buildIndexEntry(archivePath, baseURL, entry.Name())
		if nil != err {
			return nil, err
		}

		idx.Entries[indexEntry.Name] = append(idx.Entries[indexEntry.Name], *indexEntry)
	}

	// Sort entries by version descending
	for name := range idx.Entries {
		entryList := idx.Entries[name]
		sort.Slice(entryList, func(i, j int) bool {
			return entryList[i].Version > entryList[j].Version
		})
		idx.Entries[name] = entryList
	}

	return idx, nil
}

func buildIndexEntry(archivePath, baseURL, fileName string) (*IndexEntry, error) {
	digest, err := fileDigest(archivePath)
	if nil != err {
		return nil, err
	}

	meta, err := readMetadataFromArchive(archivePath)
	if nil != err {
		return nil, err
	}

	url := fileName
	if "" != baseURL {
		url = strings.TrimSuffix(baseURL, "/") + "/" + fileName
	}

	info, err := os.Stat(archivePath)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to stat archive", err)
	}

	return &IndexEntry{
		Name:        meta.Name,
		Version:     meta.Version,
		AppVersion:  meta.AppVersion,
		Description: meta.Description,
		Digest:      digest,
		URLs:        []string{url},
		Created:     info.ModTime(),
	}, nil
}

func readMetadataFromArchive(archivePath string) (*pkg.PackageMetadata, error) {
	tmpDir, err := os.MkdirTemp("", "keramos-index-*")
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to create temp directory", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := ExtractArchive(archivePath, tmpDir); nil != err {
		return nil, err
	}

	// Find the keramos.yaml — it's inside a subdirectory named after the package
	dirEntries, err := os.ReadDir(tmpDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read extracted archive", err)
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(tmpDir, entry.Name())
		meta, loadErr := pkg.LoadPackageMetadata(metaPath)
		if nil == loadErr {
			return &meta, nil
		}
	}

	return nil, keramoserr.NewError(keramoserr.ErrRepo, "archive does not contain a valid keramos.yaml")
}

func fileDigest(path string) (string, error) {
	f, err := os.Open(path)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to open file for digest", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to compute digest", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// LoadIndex reads an index.yaml from a file path.
func LoadIndex(path string) (*IndexFile, error) {
	data, err := os.ReadFile(path)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read index file", err)
	}

	var idx IndexFile
	if err := yaml.Unmarshal(data, &idx); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse index file", err)
	}

	if nil == idx.Entries {
		idx.Entries = make(map[string][]IndexEntry)
	}

	return &idx, nil
}

// SaveIndex writes an IndexFile to disk as index.yaml.
func SaveIndex(idx *IndexFile, path string) error {
	data, err := yaml.Marshal(idx)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to marshal index", err)
	}

	if err := os.WriteFile(path, data, 0644); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write index file", err)
	}

	return nil
}

// MergeIndex merges an update index into an existing one. Entries in update
// take precedence over those in existing for the same name+version.
func MergeIndex(existing, update *IndexFile) *IndexFile {
	merged := &IndexFile{
		APIVersion: "v1",
		Entries:    make(map[string][]IndexEntry),
		Generated:  time.Now(),
	}

	// Copy existing entries
	for name, entries := range existing.Entries {
		merged.Entries[name] = append(merged.Entries[name], entries...)
	}

	// Merge/override from update
	for name, updateEntries := range update.Entries {
		existingEntries := merged.Entries[name]
		for _, ue := range updateEntries {
			replaced := false
			entryLen := len(existingEntries)
			for i := 0; i < entryLen; i++ {
				if existingEntries[i].Version == ue.Version {
					existingEntries[i] = ue
					replaced = true
					break
				}
			}
			if !replaced {
				existingEntries = append(existingEntries, ue)
			}
		}
		merged.Entries[name] = existingEntries
	}

	return merged
}
