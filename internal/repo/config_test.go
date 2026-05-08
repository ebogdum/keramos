package repo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDefaultGlobalConfig(t *testing.T) {
	cfg := DefaultGlobalConfig()

	if 10 != cfg.MaxConcurrentDownloads {
		t.Errorf("expected MaxConcurrentDownloads=10, got %d", cfg.MaxConcurrentDownloads)
	}

	if 50 != cfg.RequestsPerMinute {
		t.Errorf("expected RequestsPerMinute=50, got %d", cfg.RequestsPerMinute)
	}

	if 30*time.Minute != cfg.CacheTTL {
		t.Errorf("expected CacheTTL=30m, got %s", cfg.CacheTTL)
	}

	if "" != cfg.DefaultRegistry {
		t.Errorf("expected empty DefaultRegistry, got %q", cfg.DefaultRegistry)
	}

	if "" != cfg.CAFile {
		t.Errorf("expected empty CAFile, got %q", cfg.CAFile)
	}
}

func TestGlobalConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "keramos", "config.yaml")

	cfg := &GlobalConfig{
		DefaultRegistry:        "https://registry.example.com",
		MaxConcurrentDownloads: 5,
		RequestsPerMinute:      30,
		CacheTTL:               15 * time.Minute,
		CAFile:                 "/etc/ssl/custom-ca.pem",
	}

	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); nil != err {
		t.Fatalf("failed to create dir: %v", err)
	}

	data, err := yaml.Marshal(cfg)
	if nil != err {
		t.Fatalf("failed to marshal: %v", err)
	}

	if err := os.WriteFile(configPath, data, 0644); nil != err {
		t.Fatalf("failed to write: %v", err)
	}

	// Read it back and verify
	readData, err := os.ReadFile(configPath)
	if nil != err {
		t.Fatalf("failed to read: %v", err)
	}

	var loaded GlobalConfig
	if err := yaml.Unmarshal(readData, &loaded); nil != err {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.DefaultRegistry != loaded.DefaultRegistry {
		t.Errorf("DefaultRegistry: expected %q, got %q", cfg.DefaultRegistry, loaded.DefaultRegistry)
	}

	if cfg.MaxConcurrentDownloads != loaded.MaxConcurrentDownloads {
		t.Errorf("MaxConcurrentDownloads: expected %d, got %d", cfg.MaxConcurrentDownloads, loaded.MaxConcurrentDownloads)
	}

	if cfg.RequestsPerMinute != loaded.RequestsPerMinute {
		t.Errorf("RequestsPerMinute: expected %d, got %d", cfg.RequestsPerMinute, loaded.RequestsPerMinute)
	}

	if cfg.CacheTTL != loaded.CacheTTL {
		t.Errorf("CacheTTL: expected %s, got %s", cfg.CacheTTL, loaded.CacheTTL)
	}

	if cfg.CAFile != loaded.CAFile {
		t.Errorf("CAFile: expected %q, got %q", cfg.CAFile, loaded.CAFile)
	}
}

func TestGlobalConfigYAMLRoundTrip(t *testing.T) {
	cfg := &GlobalConfig{
		DefaultRegistry:        "https://my-registry.io",
		MaxConcurrentDownloads: 8,
		RequestsPerMinute:      100,
		CacheTTL:               1 * time.Hour,
	}

	data, err := yaml.Marshal(cfg)
	if nil != err {
		t.Fatalf("marshal failed: %v", err)
	}

	var loaded GlobalConfig
	if err := yaml.Unmarshal(data, &loaded); nil != err {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cfg.MaxConcurrentDownloads != loaded.MaxConcurrentDownloads {
		t.Errorf("MaxConcurrentDownloads mismatch: %d != %d", cfg.MaxConcurrentDownloads, loaded.MaxConcurrentDownloads)
	}

	if cfg.CacheTTL != loaded.CacheTTL {
		t.Errorf("CacheTTL mismatch: %s != %s", cfg.CacheTTL, loaded.CacheTTL)
	}
}

func TestGlobalConfigDefaults(t *testing.T) {
	// Verify defaults are applied when unmarshaling an empty YAML
	data := []byte("{}")
	cfg := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, cfg); nil != err {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Defaults should be preserved since empty YAML won't override them
	if 10 != cfg.MaxConcurrentDownloads {
		t.Errorf("expected default MaxConcurrentDownloads=10, got %d", cfg.MaxConcurrentDownloads)
	}
}

func TestGlobalConfigPartialOverride(t *testing.T) {
	data := []byte("maxConcurrentDownloads: 20\n")
	cfg := DefaultGlobalConfig()
	if err := yaml.Unmarshal(data, cfg); nil != err {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if 20 != cfg.MaxConcurrentDownloads {
		t.Errorf("expected overridden MaxConcurrentDownloads=20, got %d", cfg.MaxConcurrentDownloads)
	}

	// Other defaults should remain
	if 50 != cfg.RequestsPerMinute {
		t.Errorf("expected default RequestsPerMinute=50, got %d", cfg.RequestsPerMinute)
	}
}
