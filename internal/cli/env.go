package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

// newEnvCommand prints the resolved keramos environment (paths, plugin dir, cache
// dir, namespace, kubeconfig).
func newEnvCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print keramos's environment information",
		Long:  "Print resolved environment variables and paths used by keramos.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env := collectKeramosEnv()
			keys := make([]string, 0, len(env))
			for k := range env {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%q\n", k, env[k])
			}
			return nil
		},
	}
	return cmd
}

func collectKeramosEnv() map[string]string {
	cacheRoot := os.Getenv("KERAMOS_CACHE_HOME")
	if "" == cacheRoot {
		if home, err := os.UserCacheDir(); nil == err {
			cacheRoot = filepath.Join(home, "keramos")
		}
	}
	configRoot := os.Getenv("KERAMOS_CONFIG_HOME")
	if "" == configRoot {
		if home, err := os.UserConfigDir(); nil == err {
			configRoot = filepath.Join(home, "keramos")
		}
	}
	dataRoot := os.Getenv("KERAMOS_DATA_HOME")
	if "" == dataRoot {
		if home, err := os.UserHomeDir(); nil == err {
			dataRoot = filepath.Join(home, ".local", "share", "keramos")
		}
	}
	pluginsRoot := os.Getenv("KERAMOS_PLUGINS")
	if "" == pluginsRoot {
		pluginsRoot = filepath.Join(dataRoot, "plugins")
	}

	// Order: explicit --namespace flag > KERAMOS_NAMESPACE > HELM_NAMESPACE > default.
	// HELM_NAMESPACE is honoured as a legacy fallback so existing shell
	// environments resolve to the same namespace every other keramos command uses.
	ns := namespace
	if "" == ns {
		ns = os.Getenv("KERAMOS_NAMESPACE")
	}
	if "" == ns {
		ns = os.Getenv("HELM_NAMESPACE")
	}
	if "" == ns {
		ns = "default"
	}

	bin, _ := os.Executable()
	if "" == bin {
		bin = os.Args[0]
	}

	return map[string]string{
		"KERAMOS_BIN":          bin,
		"KERAMOS_CACHE_HOME":   cacheRoot,
		"KERAMOS_CONFIG_HOME":  configRoot,
		"KERAMOS_DATA_HOME":    dataRoot,
		"KERAMOS_PLUGINS":      pluginsRoot,
		"KERAMOS_NAMESPACE":    ns,
		"KERAMOS_KUBECONFIG":   os.Getenv("KUBECONFIG"),
		"KERAMOS_KUBECONTEXT":  os.Getenv("KERAMOS_KUBECONTEXT"),
		"KERAMOS_REGISTRY_CONFIG": filepath.Join(configRoot, "registry.json"),
		"KERAMOS_REPOSITORY_CACHE": filepath.Join(cacheRoot, "repository"),
		"KERAMOS_REPOSITORY_CONFIG": filepath.Join(configRoot, "repositories.yaml"),
	}
}
