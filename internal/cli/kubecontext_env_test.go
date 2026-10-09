package cli

import (
	"bytes"
	"strings"
	"testing"
)

func runEnvCommand(t *testing.T, args ...string) string {
	t.Helper()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append(args, "env"))
	if err := root.Execute(); nil != err {
		t.Fatalf("env: %v", err)
	}
	return buf.String()
}

func TestKubeContextDefaultsFromEnv(t *testing.T) {
	t.Setenv("KERAMOS_KUBECONTEXT", "from-env")
	out := runEnvCommand(t)
	if !strings.Contains(out, `KERAMOS_KUBECONTEXT="from-env"`) {
		t.Fatalf("env did not report the context from KERAMOS_KUBECONTEXT:\n%s", out)
	}
	if "from-env" != kubeContext {
		t.Fatalf("global kube context = %q, want from-env", kubeContext)
	}
}

func TestKubeContextFlagOverridesEnv(t *testing.T) {
	t.Setenv("KERAMOS_KUBECONTEXT", "from-env")
	out := runEnvCommand(t, "--kube-context", "from-flag")
	if !strings.Contains(out, `KERAMOS_KUBECONTEXT="from-flag"`) {
		t.Fatalf("flag did not override KERAMOS_KUBECONTEXT:\n%s", out)
	}
}

func TestNamespaceDefaultsFromEnv(t *testing.T) {
	t.Setenv("KERAMOS_NAMESPACE", "ns-env")
	out := runEnvCommand(t)
	if !strings.Contains(out, `KERAMOS_NAMESPACE="ns-env"`) {
		t.Fatalf("KERAMOS_NAMESPACE ignored:\n%s", out)
	}
	if "ns-env" != namespace {
		t.Fatalf("global namespace = %q, want ns-env", namespace)
	}
}

func TestNamespaceFlagOverridesEnv(t *testing.T) {
	t.Setenv("KERAMOS_NAMESPACE", "ns-env")
	out := runEnvCommand(t, "-n", "ns-flag")
	if !strings.Contains(out, `KERAMOS_NAMESPACE="ns-flag"`) {
		t.Fatalf("-n did not override KERAMOS_NAMESPACE:\n%s", out)
	}
}

func TestDebugDefaultsFromEnv(t *testing.T) {
	t.Setenv("KERAMOS_DEBUG", "1")
	out := runEnvCommand(t)
	if !strings.Contains(out, `KERAMOS_DEBUG="true"`) {
		t.Fatalf("KERAMOS_DEBUG ignored:\n%s", out)
	}
	if !debugFlag {
		t.Fatal("global debug flag not set from KERAMOS_DEBUG")
	}
}

func TestDebugEnvRejectsNonBoolean(t *testing.T) {
	t.Setenv("KERAMOS_DEBUG", "yes-please")
	out := runEnvCommand(t)
	if !strings.Contains(out, `KERAMOS_DEBUG="false"`) {
		t.Fatalf("non-boolean KERAMOS_DEBUG must stay off:\n%s", out)
	}
}

func TestDirectoryEnvOverridesResolveEverywhere(t *testing.T) {
	t.Setenv("KERAMOS_CONFIG_HOME", "/cfg")
	t.Setenv("KERAMOS_CACHE_HOME", "/cache")
	t.Setenv("KERAMOS_DATA_HOME", "/data")
	t.Setenv("KERAMOS_PLUGINS", "")
	out := runEnvCommand(t)
	for _, line := range []string{
		`KERAMOS_CONFIG_HOME="/cfg"`,
		`KERAMOS_CACHE_HOME="/cache"`,
		`KERAMOS_DATA_HOME="/data"`,
		`KERAMOS_PLUGINS="/cfg/plugins"`,
		`KERAMOS_REGISTRY_CONFIG="/cfg/credentials.json"`,
		`KERAMOS_REPOSITORY_CONFIG="/cfg/repositories.yaml"`,
		`KERAMOS_REPOSITORY_CACHE="/cache/indexes"`,
	} {
		if !strings.Contains(out, line) {
			t.Errorf("missing %s in:\n%s", line, out)
		}
	}
	dir, err := keyringDir()
	if nil != err || "/cfg/keyring" != dir {
		t.Errorf("keyringDir = %q, %v; want /cfg/keyring", dir, err)
	}
}

func TestPluginsEnvOverridesConfigHome(t *testing.T) {
	t.Setenv("KERAMOS_CONFIG_HOME", "/cfg")
	t.Setenv("KERAMOS_PLUGINS", "/plugins")
	out := runEnvCommand(t)
	if !strings.Contains(out, `KERAMOS_PLUGINS="/plugins"`) {
		t.Fatalf("KERAMOS_PLUGINS ignored:\n%s", out)
	}
}

func TestHelmCompatDefaultsFromEnv(t *testing.T) {
	t.Setenv("KERAMOS_KUBECONTEXT", "ctx-env")
	t.Setenv("KERAMOS_NAMESPACE", "ns-env")
	for _, sub := range newHelmCompatCommand().Commands() {
		if "render" != sub.Name() && "install" != sub.Name() {
			continue
		}
		if got := sub.Flags().Lookup("kube-context").DefValue; "ctx-env" != got {
			t.Errorf("helm-compat %s --kube-context default = %q, want ctx-env", sub.Name(), got)
		}
		if got := sub.Flags().Lookup("namespace").DefValue; "ns-env" != got {
			t.Errorf("helm-compat %s --namespace default = %q, want ns-env", sub.Name(), got)
		}
	}
}
