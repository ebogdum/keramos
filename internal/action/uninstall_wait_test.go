package action

import (
	"testing"
	"time"
)

// TestWaitForDeletionSkipsKeepPolicy proves --wait does not hang on resources
// annotated resource-policy: keep (which DeleteManifests never deletes).
func TestWaitForDeletionSkipsKeepPolicy(t *testing.T) {
	client := newMockClient("apps")
	// Lookup always returns a live object → a non-skipped resource would never
	// reach remaining==0 and would time out.
	client.lookupFn = func(apiVersion, kind, namespace, name string) (map[string]any, error) {
		return map[string]any{"metadata": map[string]any{"name": name}}, nil
	}
	manifest := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: kept\n  namespace: apps\n  annotations:\n    keramos.sh/resource-policy: keep\n"
	// Short timeout: if the kept resource were waited on, this would take the
	// full timeout and error; skipping it must return immediately.
	start := time.Now()
	if err := waitForDeletion(client, manifest, "apps", 3*time.Second); nil != err {
		t.Fatalf("keep-annotated resource should be skipped, got: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 1*time.Second {
		t.Fatalf("waited on a keep resource (took %s)", elapsed)
	}
}
