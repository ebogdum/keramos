package paths

import (
	"os"
	"path/filepath"
)

const (
	ConfigHomeEnv = "KERAMOS_CONFIG_HOME"
	CacheHomeEnv  = "KERAMOS_CACHE_HOME"
	DataHomeEnv   = "KERAMOS_DATA_HOME"
	PluginsEnv    = "KERAMOS_PLUGINS"
)

func ConfigHome() (string, error) {
	return xdgHome(ConfigHomeEnv, "XDG_CONFIG_HOME", ".config")
}

func CacheHome() (string, error) {
	return xdgHome(CacheHomeEnv, "XDG_CACHE_HOME", ".cache")
}

func xdgHome(keramosEnv, xdgEnv, dotDir string) (string, error) {
	if dir := os.Getenv(keramosEnv); "" != dir {
		return dir, nil
	}
	if base := os.Getenv(xdgEnv); "" != base {
		return filepath.Join(base, "keramos"), nil
	}
	home, err := os.UserHomeDir()
	if nil != err {
		return "", err
	}
	return filepath.Join(home, dotDir, "keramos"), nil
}

func DataHome() (string, error) {
	if dir := os.Getenv(DataHomeEnv); "" != dir {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if nil != err {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "keramos"), nil
}

func Plugins() (string, error) {
	if dir := os.Getenv(PluginsEnv); "" != dir {
		return dir, nil
	}
	return underConfig("plugins")
}

func Keyring() (string, error) {
	return underConfig("keyring")
}

func RepositoryConfig() (string, error) {
	return underConfig("repositories.yaml")
}

func CredentialStore() (string, error) {
	return underConfig("credentials.json")
}

func LegacyOCICredentials() (string, error) {
	return underConfig("oci-credentials.json")
}

func MarketplaceTrustedKeys() (string, error) {
	return underConfig("marketplace_trusted_keys.json")
}

func IndexCache() (string, error) {
	return underCache("indexes")
}

func OCICache() (string, error) {
	return underCache("oci")
}

func GitCache() (string, error) {
	return underCache("git")
}

func RegistryCache() (string, error) {
	return underCache("registry")
}

func underConfig(name string) (string, error) {
	root, err := ConfigHome()
	if nil != err {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func underCache(name string) (string, error) {
	root, err := CacheHome()
	if nil != err {
		return "", err
	}
	return filepath.Join(root, name), nil
}
