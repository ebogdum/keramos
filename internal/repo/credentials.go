package repo

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/logger"
)

// AuthType identifies the authentication method for a credential.
type AuthType string

const (
	// AuthBasic is HTTP Basic authentication (username + password).
	AuthBasic AuthType = "basic"
	// AuthBearer is a bearer token in the Authorization header.
	AuthBearer AuthType = "bearer"
	// AuthAPIKey is an API key sent in the X-API-Key header.
	AuthAPIKey AuthType = "apikey"
)

// Credential holds authentication details for a single host.
type Credential struct {
	Type     AuthType `json:"type"`
	Username string   `json:"username,omitempty"`
	Password string   `json:"password,omitempty"`
	Token    string   `json:"token,omitempty"`
	APIKey   string   `json:"apiKey,omitempty"`
	// Insecure marks this host as one the operator has opted to reach over an
	// untrusted transport (bad/self-signed TLS or plain HTTP). Set via
	// `keramos login --insecure`; honored by the registry/HTTP clients for this host.
	Insecure bool `json:"insecure,omitempty"`
}

// CredentialStore manages credentials for multiple hosts.
type CredentialStore struct {
	Credentials map[string]Credential `json:"credentials"`
	CredHelpers map[string]string     `json:"credHelpers,omitempty"`
	path        string
	helperCache map[string]Credential
}

func credentialStorePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrAuth, "failed to determine config directory", err)
	}
	return filepath.Join(configDir, "keramos", "credentials.json"), nil
}

// LoadCredentialStore reads credentials from ~/.config/keramos/credentials.json.
// It creates the file if missing and migrates from oci-credentials.json when present.
func LoadCredentialStore() (*CredentialStore, error) {
	path, err := credentialStorePath()
	if nil != err {
		return nil, err
	}

	store := &CredentialStore{
		Credentials: make(map[string]Credential),
		path:        path,
	}

	data, err := os.ReadFile(path)
	if nil != err {
		if !os.IsNotExist(err) {
			return nil, keramoserr.WrapError(keramoserr.ErrAuth, "failed to read credentials file", err)
		}
		// File doesn't exist — try migrating from legacy OCI credentials
		if migErr := migrateOCICredentials(store); nil != migErr {
			logger.Debug("OCI credential migration skipped: %v", migErr)
		}
		return store, nil
	}

	if err := json.Unmarshal(data, store); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrAuth, "failed to parse credentials file", err)
	}

	if nil == store.Credentials {
		store.Credentials = make(map[string]Credential)
	}

	return store, nil
}

// Save writes the credential store to disk with restricted permissions.
func (cs *CredentialStore) Save() error {
	path := cs.path
	if "" == path {
		var err error
		path, err = credentialStorePath()
		if nil != err {
			return err
		}
		cs.path = path
	}

	dir := filepath.Dir(path)
	// Use 0o700 so credentials.json's parent isn't world-traversable on
	// shared systems. The credentials file itself is 0o600.
	if err := os.MkdirAll(dir, 0o700); nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "failed to create config directory", err)
	}

	data, err := json.MarshalIndent(cs, "", "  ")
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "failed to marshal credentials", err)
	}

	if err := os.WriteFile(path, data, 0600); nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "failed to write credentials file", err)
	}

	return nil
}

// Set adds or updates a credential for the given host.
func (cs *CredentialStore) Set(host string, cred Credential) {
	cs.Credentials[host] = cred
}

// Remove deletes the credential for the given host.
func (cs *CredentialStore) Remove(host string) {
	delete(cs.Credentials, host)
}

// Get returns the credential for an exact host match.
func (cs *CredentialStore) Get(host string) (Credential, bool) {
	c, ok := cs.Credentials[host]
	return c, ok
}

// GetForHost tries an exact credential match first, then falls back to credential helpers.
// For credential helpers, it checks both exact matches and glob patterns on hostnames,
// and caches results for the lifetime of the store instance.
func (cs *CredentialStore) GetForHost(host string) (Credential, bool) {
	if c, ok := cs.Credentials[host]; ok {
		return c, true
	}

	if nil == cs.CredHelpers {
		return Credential{}, false
	}

	// Check session cache first
	if nil != cs.helperCache {
		if cached, ok := cs.helperCache[host]; ok {
			return cached, true
		}
	}

	helper := matchCredHelper(cs.CredHelpers, host)
	if "" == helper {
		return Credential{}, false
	}

	cred, err := execCredentialHelper(helper, host)
	if nil != err {
		logger.Debug("credential helper %q failed for %s: %v", helper, host, err)
		return Credential{}, false
	}

	// Cache for the session
	if nil == cs.helperCache {
		cs.helperCache = make(map[string]Credential)
	}
	cs.helperCache[host] = cred

	return cred, true
}

// matchCredHelper finds a credential helper for the host, checking exact match first,
// then glob patterns.
func matchCredHelper(helpers map[string]string, host string) string {
	// Exact match takes priority
	if helper, ok := helpers[host]; ok {
		return helper
	}

	// Try glob matching on patterns
	for pattern, helper := range helpers {
		if !strings.ContainsAny(pattern, "*?[") {
			continue
		}
		matched, err := filepath.Match(pattern, host)
		if nil == err && matched {
			return helper
		}
	}

	return ""
}

type credHelperResponse struct {
	Username string `json:"Username"`
	Secret   string `json:"Secret"`
}

func execCredentialHelper(helper, host string) (Credential, error) {
	binary := "docker-credential-" + helper

	cmd := exec.Command(binary, "get")
	cmd.Stdin = strings.NewReader(host)

	out, err := cmd.Output()
	if nil != err {
		return Credential{}, keramoserr.WrapErrorf(keramoserr.ErrAuth, err, "credential helper %q failed", binary)
	}

	var resp credHelperResponse
	if err := json.Unmarshal(out, &resp); nil != err {
		return Credential{}, keramoserr.WrapError(keramoserr.ErrAuth, "failed to parse credential helper response", err)
	}

	return Credential{
		Type:     AuthBasic,
		Username: resp.Username,
		Password: resp.Secret,
	}, nil
}

func migrateOCICredentials(store *CredentialStore) error {
	configDir, err := os.UserConfigDir()
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "failed to determine config directory", err)
	}

	legacyPath := filepath.Join(configDir, "keramos", "oci-credentials.json")
	data, err := os.ReadFile(legacyPath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "no legacy OCI credentials to migrate", err)
	}

	var legacy map[string]ociCredential
	if err := json.Unmarshal(data, &legacy); nil != err {
		return keramoserr.WrapError(keramoserr.ErrAuth, "failed to parse legacy OCI credentials", err)
	}

	for host, cred := range legacy {
		store.Credentials[host] = Credential{
			Type:     AuthBasic,
			Username: cred.Username,
			Password: cred.Password,
		}
	}

	if 0 < len(legacy) {
		if err := store.Save(); nil != err {
			return err
		}
		logger.Debug("migrated %d OCI credentials to unified store", len(legacy))
	}

	return nil
}
