package repo

import (
	"os"
	"path/filepath"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"gopkg.in/yaml.v3"
)

// GlobalConfig holds keramos-wide configuration settings.
type GlobalConfig struct {
	DefaultRegistry        string        `yaml:"defaultRegistry,omitempty"`
	MaxConcurrentDownloads int           `yaml:"maxConcurrentDownloads"`
	RequestsPerMinute      int           `yaml:"requestsPerMinute"`
	CacheTTL               time.Duration `yaml:"cacheTTL"`
	CAFile                 string        `yaml:"caFile,omitempty"`
}

// DefaultGlobalConfig returns a GlobalConfig with sensible defaults.
func DefaultGlobalConfig() *GlobalConfig {
	return &GlobalConfig{
		MaxConcurrentDownloads: 10,
		RequestsPerMinute:      50,
		CacheTTL:               30 * time.Minute,
	}
}

func globalConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrRepo, "failed to determine config directory", err)
	}

	return filepath.Join(configDir, "keramos", "config.yaml"), nil
}

// LoadGlobalConfig loads the global configuration from ~/.config/keramos/config.yaml.
// Returns defaults if the file does not exist.
func LoadGlobalConfig() (*GlobalConfig, error) {
	path, err := globalConfigPath()
	if nil != err {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if nil != err {
		if os.IsNotExist(err) {
			return DefaultGlobalConfig(), nil
		}
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to read global config", err)
	}

	cfg := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, cfg); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrRepo, "failed to parse global config", err)
	}

	return cfg, nil
}

// Save writes the global configuration to disk.
func (c *GlobalConfig) Save() error {
	path, err := globalConfigPath()
	if nil != err {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to create config directory", err)
	}

	data, err := yaml.Marshal(c)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to marshal global config", err)
	}

	if err := os.WriteFile(path, data, 0600); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write global config", err)
	}

	return nil
}
