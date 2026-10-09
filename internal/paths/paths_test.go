package paths

import (
	"path/filepath"
	"testing"
)

func TestEnvOverridesWin(t *testing.T) {
	t.Setenv(ConfigHomeEnv, "/cfg")
	t.Setenv(CacheHomeEnv, "/cache")
	t.Setenv(DataHomeEnv, "/data")
	t.Setenv(PluginsEnv, "/plugins")
	cases := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{"ConfigHome", ConfigHome, "/cfg"},
		{"CacheHome", CacheHome, "/cache"},
		{"DataHome", DataHome, "/data"},
		{"Plugins", Plugins, "/plugins"},
		{"Keyring", Keyring, "/cfg/keyring"},
		{"RepositoryConfig", RepositoryConfig, "/cfg/repositories.yaml"},
		{"CredentialStore", CredentialStore, "/cfg/credentials.json"},
		{"LegacyOCICredentials", LegacyOCICredentials, "/cfg/oci-credentials.json"},
		{"MarketplaceTrustedKeys", MarketplaceTrustedKeys, "/cfg/marketplace_trusted_keys.json"},
		{"IndexCache", IndexCache, "/cache/indexes"},
		{"OCICache", OCICache, "/cache/oci"},
		{"GitCache", GitCache, "/cache/git"},
		{"RegistryCache", RegistryCache, "/cache/registry"},
	}
	for _, c := range cases {
		got, err := c.fn()
		if nil != err {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if filepath.FromSlash(c.want) != got {
			t.Errorf("%s = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestPluginsFallsBackUnderConfigHome(t *testing.T) {
	t.Setenv(ConfigHomeEnv, "/cfg")
	t.Setenv(PluginsEnv, "")
	got, err := Plugins()
	if nil != err || filepath.FromSlash("/cfg/plugins") != got {
		t.Fatalf("Plugins = %q, %v", got, err)
	}
}

func TestDefaultsLiveUnderHome(t *testing.T) {
	t.Setenv("HOME", "/home/u")
	t.Setenv(ConfigHomeEnv, "")
	t.Setenv(CacheHomeEnv, "")
	t.Setenv(DataHomeEnv, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	for name, c := range map[string]struct {
		fn   func() (string, error)
		want string
	}{
		"ConfigHome": {ConfigHome, "/home/u/.config/keramos"},
		"CacheHome":  {CacheHome, "/home/u/.cache/keramos"},
		"DataHome":   {DataHome, "/home/u/.local/share/keramos"},
	} {
		got, err := c.fn()
		if nil != err {
			t.Fatalf("%s: %v", name, err)
		}
		if filepath.FromSlash(c.want) != got {
			t.Errorf("%s = %q, want %q", name, got, c.want)
		}
	}
}

func TestXDGBaseDirsRespected(t *testing.T) {
	t.Setenv(ConfigHomeEnv, "")
	t.Setenv(CacheHomeEnv, "")
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	cfg, _ := ConfigHome()
	cache, _ := CacheHome()
	if filepath.FromSlash("/xdg/config/keramos") != cfg || filepath.FromSlash("/xdg/cache/keramos") != cache {
		t.Fatalf("ConfigHome=%q CacheHome=%q", cfg, cache)
	}
}
