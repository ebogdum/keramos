package repo

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/logger"
	"gopkg.in/yaml.v3"
)

const (
	// maxIndexSize is the maximum allowed size for a remote index.yaml (50 MB).
	maxIndexSize = 50 * 1024 * 1024
	// maxArchiveSize bounds a downloaded package archive (1 GB) so a hostile
	// registry cannot exhaust the disk before extraction caps apply.
	maxArchiveSize = 1024 * 1024 * 1024
)

var (
	defaultClient   *AuthenticatedClient
	defaultClientMu sync.Mutex
)

// DefaultClient returns the package-level authenticated HTTP client.
// It lazily initializes the client on first use.
func DefaultClient() (*AuthenticatedClient, error) {
	defaultClientMu.Lock()
	defer defaultClientMu.Unlock()

	if nil != defaultClient {
		return defaultClient, nil
	}

	store, err := LoadCredentialStore()
	if nil != err {
		return nil, err
	}

	client, err := NewAuthenticatedClient(store)
	if nil != err {
		return nil, err
	}

	defaultClient = client
	return defaultClient, nil
}

// SetDefaultClient replaces the package-level authenticated HTTP client.
func SetDefaultClient(c *AuthenticatedClient) {
	defaultClientMu.Lock()
	defer defaultClientMu.Unlock()
	defaultClient = c
}

// RepoConfig represents a configured repository.
type RepoConfig struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	CAFile   string `yaml:"caFile,omitempty"`
	CertFile string `yaml:"certFile,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty"`
	// InsecureSkipTLSVerify disables server-certificate verification for
	// fetches from this repo. Opt-in via `repo add --insecure-skip-tls-verify`.
	InsecureSkipTLSVerify bool `yaml:"insecureSkipTLSVerify,omitempty"`
	// PassCredentials forwards the repo's credentials to a redirect target on
	// the first hop even when the host differs. PassCredentialsAll forwards on
	// every redirect hop. Both relax the default (cross-host redirects blocked).
	PassCredentials    bool `yaml:"passCredentials,omitempty"`
	PassCredentialsAll bool `yaml:"passCredentialsAll,omitempty"`
}

// RepoFile holds the list of configured repositories.
// Stored at ~/.config/keramos/repositories.yaml
type RepoFile struct {
	Repositories []RepoConfig `yaml:"repositories"`
}

// Add adds a repository. Returns an error if the name already exists.
func (rf *RepoFile) Add(name, url string) error {
	for _, r := range rf.Repositories {
		if r.Name == name {
			return keramoserr.NewErrorf(keramoserr.ErrRepo, "repository %q already exists", name)
		}
	}

	rf.Repositories = append(rf.Repositories, RepoConfig{Name: name, URL: url})
	return nil
}

// Remove removes a repository by name.
// Find returns a pointer to the matching repo entry or nil. The pointer
// references the slice element, so callers can mutate fields in place.
func (rf *RepoFile) Find(name string) *RepoConfig {
	for i := range rf.Repositories {
		if rf.Repositories[i].Name == name {
			return &rf.Repositories[i]
		}
	}
	return nil
}

func (rf *RepoFile) Remove(name string) error {
	repoLen := len(rf.Repositories)
	for i := 0; i < repoLen; i++ {
		if rf.Repositories[i].Name == name {
			rf.Repositories = append(rf.Repositories[:i], rf.Repositories[i+1:]...)
			return nil
		}
	}

	return keramoserr.NewErrorf(keramoserr.ErrRepo, "repository %q not found", name)
}

func repoFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to determine config directory", err)
	}

	return filepath.Join(configDir, "keramos", "repositories.yaml"), nil
}

// LoadRepoFile loads repositories from the config file.
func LoadRepoFile() (*RepoFile, error) {
	path, err := repoFilePath()
	if nil != err {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if nil != err {
		if os.IsNotExist(err) {
			return &RepoFile{}, nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read repository config", err)
	}

	var rf RepoFile
	if err := yaml.Unmarshal(data, &rf); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse repository config", err)
	}

	return &rf, nil
}

// Save writes the repository config to disk.
func (rf *RepoFile) Save() error {
	path, err := repoFilePath()
	if nil != err {
		return err
	}

	dir := filepath.Dir(path)
	// 0o700: avoid leaking the existence of the config directory to other
	// local users; repositories.yaml may carry mTLS file paths.
	if err := os.MkdirAll(dir, 0o700); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to create config directory", err)
	}

	data, err := yaml.Marshal(rf)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to marshal repository config", err)
	}

	if err := os.WriteFile(path, data, 0600); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write repository config", err)
	}

	return nil
}

// FetchIndex downloads the index.yaml from a repository URL.
func FetchIndex(repoURL string) (*IndexFile, error) {
	client, err := ClientForURL(repoURL)
	if nil != err {
		return nil, err
	}
	return FetchIndexWith(client, repoURL)
}

// FetchIndexWith downloads the index.yaml using the provided client. If the
// URL scheme is handled by a downloader plugin (e.g. s3://, gs://) the plugin
// is invoked instead of the HTTP client.
func FetchIndexWith(client *AuthenticatedClient, repoURL string) (*IndexFile, error) {
	indexURL := strings.TrimSuffix(repoURL, "/") + "/index.yaml"
	logger.Debug("fetching index from %s", indexURL)

	if data, ok, dlErr := tryDownloaderFetch(indexURL); ok {
		if nil != dlErr {
			return nil, dlErr
		}
		var idx IndexFile
		if err := yaml.Unmarshal(data, &idx); nil != err {
			return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse remote index", err)
		}
		if nil == idx.Entries {
			idx.Entries = make(map[string][]IndexEntry)
		}
		return &idx, nil
	}

	resp, err := client.Get(indexURL)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to fetch repository index", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, keramoserr.NewErrorf(keramoserr.ErrRepo, "failed to fetch index: HTTP %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, maxIndexSize+1)
	data, err := io.ReadAll(limitedReader)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read index response", err)
	}
	if int64(len(data)) > maxIndexSize {
		return nil, keramoserr.NewErrorf(keramoserr.ErrRepo, "remote index exceeds maximum allowed size of %d bytes", maxIndexSize)
	}

	var idx IndexFile
	if err := yaml.Unmarshal(data, &idx); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse remote index", err)
	}

	if nil == idx.Entries {
		idx.Entries = make(map[string][]IndexEntry)
	}

	return &idx, nil
}

// DownloadPackage downloads a package archive from a repository.
// It returns the path to the downloaded file in a temp directory.
func DownloadPackage(repoURL, name, version string) (string, error) {
	client, err := ClientForURL(repoURL)
	if nil != err {
		return "", err
	}
	return DownloadPackageWith(client, repoURL, name, version)
}

// DownloadPackageWith downloads a package archive using the provided client.
func DownloadPackageWith(client *AuthenticatedClient, repoURL, name, version string) (string, error) {
	idx, err := FetchIndexWith(client, repoURL)
	if nil != err {
		return "", err
	}

	entries, ok := idx.Entries[name]
	if !ok {
		return "", keramoserr.NewErrorf(keramoserr.ErrRepo, "package %q not found in repository", name)
	}

	var matchedEntry *IndexEntry
	for i := range entries {
		if entries[i].Version == version {
			matchedEntry = &entries[i]
			break
		}
	}

	if nil == matchedEntry {
		return "", keramoserr.NewErrorf(keramoserr.ErrRepo, "version %q of package %q not found", version, name)
	}

	if 0 == len(matchedEntry.URLs) {
		return "", keramoserr.NewErrorf(keramoserr.ErrRepo, "no download URL for %s-%s", name, version)
	}

	downloadURL := matchedEntry.URLs[0]
	if !strings.HasPrefix(downloadURL, "http://") && !strings.HasPrefix(downloadURL, "https://") {
		downloadURL = strings.TrimSuffix(repoURL, "/") + "/" + downloadURL
	}

	if err := validateDownloadHost(repoURL, downloadURL); nil != err {
		return "", err
	}

	return downloadArchive(client, downloadURL, ArchiveFileName(name, version))
}

// validateDownloadHost ensures the download URL host matches the repository URL host,
// preventing SSRF attacks via malicious index entries.
func validateDownloadHost(repoURL, downloadURL string) error {
	repoParsed, err := url.Parse(repoURL)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse repository URL", err)
	}

	dlParsed, err := url.Parse(downloadURL)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse download URL", err)
	}

	if repoParsed.Host != dlParsed.Host {
		return keramoserr.NewErrorf(keramoserr.ErrRepo,
			"download URL host %q does not match repository host %q", dlParsed.Host, repoParsed.Host)
	}

	return nil
}

// DownloadArchive fetches an archive from any URL, returning the local path
// to the downloaded file. The caller owns the file and is responsible for
// removing it. Downloader plugins are consulted for non-http(s) schemes.
//
// SECURITY: rejects http:// for the default HTTP path so credentials never
// flow over plaintext, and refuses link-local / metadata-service hosts to
// block SSRF. Downloader plugins are still consulted for opaque schemes
// (s3://, gcs://) — they are responsible for their own host policy.
func DownloadArchive(rawURL string) (string, error) {
	if data, ok, dlErr := tryDownloaderFetch(rawURL); ok {
		if nil != dlErr {
			return "", dlErr
		}
		tmp, err := os.CreateTemp("", "keramos-archive-*.tgz")
		if nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to create temp archive", err)
		}
		if _, writeErr := tmp.Write(data); nil != writeErr {
			tmp.Close()
			_ = os.Remove(tmp.Name())
			return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to write downloaded archive", writeErr)
		}
		if err := tmp.Close(); nil != err {
			return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to close temp archive", err)
		}
		return tmp.Name(), nil
	}

	if err := validateHTTPSURL(rawURL); nil != err {
		return "", err
	}
	client, err := ClientForURL(rawURL)
	if nil != err {
		return "", err
	}
	fileName := filepath.Base(rawURL)
	if "" == fileName || "." == fileName || "/" == fileName {
		fileName = "package.keramos.tgz"
	}
	return downloadArchive(client, rawURL, fileName)
}

// validateHTTPSURL refuses non-HTTPS URLs and well-known SSRF targets.
func validateHTTPSURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrRepo, err, "invalid URL %q", rawURL)
	}
	scheme := strings.ToLower(u.Scheme)
	if "https" != scheme {
		return keramoserr.NewErrorf(keramoserr.ErrRepo,
			"refusing to download over %q (only https:// allowed for safety)", scheme)
	}
	host := u.Hostname()
	// Block obvious SSRF targets. Operators that need internal HTTPS
	// repositories should configure them explicitly via repo.RepoConfig
	// where the URL is recorded as trusted at registration time.
	if "169.254.169.254" == host || "metadata.google.internal" == host || "metadata" == host {
		return keramoserr.NewErrorf(keramoserr.ErrRepo,
			"refusing to fetch from cloud metadata service host %q", host)
	}
	return nil
}

// VerifyArchive looks for a `<archive>.prov` sidecar at the same URL and
// verifies its PGP signature against the user's keyring. Returns an error
// when the prov file is missing or the signature does not validate.
func VerifyArchive(archivePath, archiveURL string) error {
	provURL := archiveURL + ".prov"
	provLocal, err := DownloadArchive(provURL)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrSignature, err, "could not download provenance %s", provURL)
	}
	defer os.Remove(provLocal)

	home, _ := os.UserHomeDir()
	keyringDir := filepath.Join(home, ".config", "keramos", "keyring")
	return VerifySignatureFromKeyring(archivePath, provLocal, keyringDir)
}

func downloadArchive(client *AuthenticatedClient, archiveURL, fileName string) (string, error) {
	logger.Debug("downloading %s", archiveURL)

	resp, err := client.Get(archiveURL)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to download package", err)
	}
	defer resp.Body.Close()

	if 200 != resp.StatusCode {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", keramoserr.NewErrorf(keramoserr.ErrRepo, "download failed: HTTP %d", resp.StatusCode)
	}

	tmpDir, err := os.MkdirTemp("", "keramos-download-*")
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to create temp directory", err)
	}

	success := false
	defer func() {
		if !success {
			os.RemoveAll(tmpDir)
		}
	}()

	destPath := filepath.Join(tmpDir, fileName)
	outFile, err := os.Create(destPath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to create output file", err)
	}
	defer outFile.Close()

	// Bound the download: a hostile registry could otherwise stream an
	// arbitrarily large (or never-ending) body and exhaust the disk before the
	// extractor's per-file/total caps ever run. Mirror the index-fetch cap.
	written, err := io.Copy(outFile, io.LimitReader(resp.Body, maxArchiveSize+1))
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to write downloaded package", err)
	}
	if written > maxArchiveSize {
		return "", keramoserr.NewErrorf(keramoserr.ErrRepo,
			"downloaded package exceeds maximum allowed size of %d bytes", maxArchiveSize)
	}

	success = true
	return destPath, nil
}
