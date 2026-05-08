package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/release"
)

func kubeconfig(t *testing.T) string {
	t.Helper()
	kc := os.Getenv("KUBECONFIG")
	if "" == kc {
		t.Skip("KUBECONFIG not set — run e2e/setup.sh first")
	}
	if _, err := os.Stat(kc); nil != err {
		t.Skipf("KUBECONFIG file not found: %s", kc)
	}
	return kc
}

func newClient(t *testing.T, ns string) kube.KubeClient {
	t.Helper()
	kc := kubeconfig(t)
	client, err := kube.NewClient(kc, "", ns)
	if nil != err {
		t.Fatalf("failed to create kube client: %v", err)
	}
	client.SetTimeout(30 * time.Second)
	return client
}

func fixturesDir() string {
	return filepath.Join("..", "test", "fixtures", "simple")
}

// TestFullLifecycle runs install → status → upgrade → rollback → uninstall against a real cluster.
func TestFullLifecycle(t *testing.T) {
	client := newClient(t, "keramos-e2e")
	pkgPath := fixturesDir()
	releaseName := "e2e-test"

	// Clean up at end regardless
	defer func() {
		_, _ = action.Uninstall(client, &action.UninstallOptions{
			ReleaseName: releaseName,
			Namespace:   "keramos-e2e",
			KeepHistory: false,
			Timeout:     30 * time.Second,
		})
		// Delete the namespace
		kubectl(t, "delete", "namespace", "keramos-e2e", "--ignore-not-found")
	}()

	// --- INSTALL ---
	t.Run("Install", func(t *testing.T) {
		rel, err := action.Install(client, pkgPath, &action.InstallOptions{
			ReleaseName:     releaseName,
			Namespace:       "keramos-e2e",
			Wait:            true,
			Timeout:         60 * time.Second,
			DryRun:          "",
			Profile:         "",
		})
		if nil != err {
			t.Fatalf("Install failed: %v", err)
		}
		if release.StatusDeployed != rel.Status {
			t.Errorf("expected status deployed, got %s", rel.Status)
		}
		if 1 != rel.Revision {
			t.Errorf("expected revision 1, got %d", rel.Revision)
		}
		t.Logf("Installed release %s revision %d", rel.Name, rel.Revision)

		// Verify resources exist in cluster
		assertResourceExists(t, "keramos-e2e", "deployment", "myapp")
		assertResourceExists(t, "keramos-e2e", "service", "myapp")
	})

	// --- STATUS ---
	t.Run("Status", func(t *testing.T) {
		storage := release.NewSecretStorage(client.Clientset(), "keramos-e2e")
		rel, err := storage.Last(releaseName)
		if nil != err {
			t.Fatalf("Failed to get release: %v", err)
		}
		if release.StatusDeployed != rel.Status {
			t.Errorf("expected deployed, got %s", rel.Status)
		}
		if "simple-app" != rel.Package.Name {
			t.Errorf("expected package name simple-app, got %s", rel.Package.Name)
		}
	})

	// --- UPGRADE ---
	t.Run("Upgrade", func(t *testing.T) {
		rel, err := action.Upgrade(client, pkgPath, &action.UpgradeOptions{
			ReleaseName: releaseName,
			Namespace:   "keramos-e2e",
			Sets:        []string{"replicas=5"},
			Wait:        true,
			Timeout:     60 * time.Second,
			DryRun:      "",
		})
		if nil != err {
			t.Fatalf("Upgrade failed: %v", err)
		}
		if release.StatusDeployed != rel.Status {
			t.Errorf("expected status deployed, got %s", rel.Status)
		}
		if 2 != rel.Revision {
			t.Errorf("expected revision 2, got %d", rel.Revision)
		}
		t.Logf("Upgraded release %s to revision %d", rel.Name, rel.Revision)

		// Verify the deployment has 5 replicas
		out := kubectl(t, "get", "deployment", "myapp", "-n", "keramos-e2e", "-o", "jsonpath={.spec.replicas}")
		if "5" != strings.TrimSpace(out) {
			t.Errorf("expected 5 replicas, got %s", out)
		}
	})

	// --- HISTORY ---
	t.Run("History", func(t *testing.T) {
		storage := release.NewSecretStorage(client.Clientset(), "keramos-e2e")
		history, err := storage.History(releaseName)
		if nil != err {
			t.Fatalf("History failed: %v", err)
		}
		if 2 != len(history) {
			t.Errorf("expected 2 revisions, got %d", len(history))
		}
	})

	// --- ROLLBACK ---
	t.Run("Rollback", func(t *testing.T) {
		rel, err := action.Rollback(client, &action.RollbackOptions{
			ReleaseName: releaseName,
			Namespace:   "keramos-e2e",
			Revision:    1,
			Wait:        true,
			Timeout:     60 * time.Second,
		})
		if nil != err {
			t.Fatalf("Rollback failed: %v", err)
		}
		if release.StatusDeployed != rel.Status {
			t.Errorf("expected status deployed, got %s", rel.Status)
		}
		if 3 != rel.Revision {
			t.Errorf("expected revision 3, got %d", rel.Revision)
		}
		t.Logf("Rolled back to revision %d (new rev %d)", 1, rel.Revision)

		// Verify replicas back to 3 (original value)
		out := kubectl(t, "get", "deployment", "myapp", "-n", "keramos-e2e", "-o", "jsonpath={.spec.replicas}")
		if "3" != strings.TrimSpace(out) {
			t.Errorf("expected 3 replicas after rollback, got %s", out)
		}
	})

	// --- UNINSTALL ---
	t.Run("Uninstall", func(t *testing.T) {
		_, err := action.Uninstall(client, &action.UninstallOptions{
			ReleaseName: releaseName,
			Namespace:   "keramos-e2e",
			KeepHistory: false,
			Timeout:     30 * time.Second,
		})
		if nil != err {
			t.Fatalf("Uninstall failed: %v", err)
		}
		t.Log("Uninstalled release")

		// Verify resources are gone
		assertResourceNotExists(t, "keramos-e2e", "deployment", "myapp")
		assertResourceNotExists(t, "keramos-e2e", "service", "myapp")

		// Verify release secret is gone
		storage := release.NewSecretStorage(client.Clientset(), "keramos-e2e")
		_, err = storage.Last(releaseName)
		if nil == err {
			t.Error("expected release to be deleted from storage")
		}
	})
}

// TestDryRunClient verifies client-side dry-run doesn't touch the cluster.
func TestDryRunClient(t *testing.T) {
	client := newClient(t, "keramos-e2e-dry")
	pkgPath := fixturesDir()

	rel, err := action.Install(client, pkgPath, &action.InstallOptions{
		ReleaseName: "dry-test",
		Namespace:   "keramos-e2e-dry",
		DryRun:      "client",
	})
	if nil != err {
		t.Fatalf("Dry-run install failed: %v", err)
	}
	if "" == rel.Manifest {
		t.Error("expected rendered manifest in dry-run result")
	}
	if release.StatusDeployed == rel.Status {
		t.Error("dry-run should not produce deployed status")
	}

	// Verify nothing was created
	assertResourceNotExists(t, "keramos-e2e-dry", "deployment", "myapp")
}

// TestDryRunServer verifies server-side dry-run validates but doesn't persist.
func TestDryRunServer(t *testing.T) {
	client := newClient(t, "keramos-e2e-server")
	pkgPath := fixturesDir()

	// Create namespace first since server dry-run needs it
	kubectl(t, "create", "namespace", "keramos-e2e-server", "--dry-run=client", "-o", "yaml")
	_ = client.CreateNamespace("keramos-e2e-server")

	defer kubectl(t, "delete", "namespace", "keramos-e2e-server", "--ignore-not-found")

	rel, err := action.Install(client, pkgPath, &action.InstallOptions{
		ReleaseName: "server-dry-test",
		Namespace:   "keramos-e2e-server",
		DryRun:      "server",
	})
	if nil != err {
		t.Fatalf("Server dry-run install failed: %v", err)
	}
	if "" == rel.Manifest {
		t.Error("expected rendered manifest")
	}

	// Verify nothing was persisted
	assertResourceNotExists(t, "keramos-e2e-server", "deployment", "myapp")
}

// TestInstallWithProfiles tests base+profile layering against a real cluster.
func TestInstallWithProfiles(t *testing.T) {
	client := newClient(t, "keramos-e2e-profile")
	pkgPath := filepath.Join("..", "test", "fixtures", "with-profiles")
	releaseName := "profile-test"

	defer func() {
		_, _ = action.Uninstall(client, &action.UninstallOptions{
			ReleaseName: releaseName,
			Namespace:   "keramos-e2e-profile",
			Timeout:     30 * time.Second,
		})
		kubectl(t, "delete", "namespace", "keramos-e2e-profile", "--ignore-not-found")
	}()

	rel, err := action.Install(client, pkgPath, &action.InstallOptions{
		ReleaseName:     releaseName,
		Namespace:       "keramos-e2e-profile",
		Profile:         "prod",
		Wait:            true,
		Timeout:         60 * time.Second,
	})
	if nil != err {
		t.Fatalf("Install with profile failed: %v", err)
	}
	if release.StatusDeployed != rel.Status {
		t.Errorf("expected deployed, got %s", rel.Status)
	}

	// Verify prod profile applied (replicas=5)
	out := kubectl(t, "get", "deployment", "myapp", "-n", "keramos-e2e-profile", "-o", "jsonpath={.spec.replicas}")
	if "5" != strings.TrimSpace(out) {
		t.Errorf("expected 5 replicas from prod profile, got %s", out)
	}
}

// --- helpers ---

func kubectl(t *testing.T, args ...string) string {
	t.Helper()
	kc := kubeconfig(t)
	cmdArgs := append([]string{"--kubeconfig", kc}, args...)
	cmd := exec.Command("kubectl", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if nil != err {
		// Don't fail for expected failures (like checking non-existence)
		return string(out)
	}
	return string(out)
}

func assertResourceExists(t *testing.T, ns, kind, name string) {
	t.Helper()
	out := kubectl(t, "get", kind, name, "-n", ns, "--no-headers")
	if strings.Contains(out, "not found") || strings.Contains(out, "NotFound") {
		t.Errorf("expected %s/%s to exist in %s, but it was not found", kind, name, ns)
	}
}

func assertResourceNotExists(t *testing.T, ns, kind, name string) {
	t.Helper()
	out := kubectl(t, "get", kind, name, "-n", ns, "--no-headers", "--ignore-not-found")
	if "" != strings.TrimSpace(out) {
		t.Errorf("expected %s/%s to not exist in %s, but found: %s", kind, name, ns, out)
	}
}
