package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/ebogdum/keramos/v3/internal/paths"
	"github.com/spf13/cobra"
)

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
	ns := namespace
	if "" == ns {
		ns = namespaceFromEnv()
	}
	if "" == ns {
		ns = "default"
	}

	bin, _ := os.Executable()
	if "" == bin {
		bin = os.Args[0]
	}

	return map[string]string{
		"KERAMOS_BIN":               bin,
		"KERAMOS_CACHE_HOME":        resolvedPath(paths.CacheHome),
		"KERAMOS_CONFIG_HOME":       resolvedPath(paths.ConfigHome),
		"KERAMOS_DATA_HOME":         resolvedPath(paths.DataHome),
		"KERAMOS_DEBUG":             fmt.Sprintf("%t", debugFlag),
		"KERAMOS_PLUGINS":           resolvedPath(paths.Plugins),
		"KERAMOS_NAMESPACE":         ns,
		"KERAMOS_KUBECONFIG":        os.Getenv("KUBECONFIG"),
		"KERAMOS_KUBECONTEXT":       kubeContext,
		"KERAMOS_REGISTRY_CONFIG":   resolvedPath(paths.CredentialStore),
		"KERAMOS_REPOSITORY_CACHE":  resolvedPath(paths.IndexCache),
		"KERAMOS_REPOSITORY_CONFIG": resolvedPath(paths.RepositoryConfig),
	}
}

func resolvedPath(resolve func() (string, error)) string {
	dir, err := resolve()
	if nil != err {
		return ""
	}
	return dir
}
