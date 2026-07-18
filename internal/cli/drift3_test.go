package cli

import (
	"strings"
	"testing"
)

const (
	svcPkg = "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n  namespace: apps\n  labels:\n    tier: new\nspec:\n  ports:\n    - port: 8080\n"
	svcSt  = "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n  namespace: apps\n  labels:\n    tier: old\nspec:\n  ports:\n    - port: 8080\n"
	svcRun = "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n  namespace: apps\n  labels:\n    tier: old\nspec:\n  ports:\n    - port: 9090\n"
)

// TestThreeWayClassifies verifies the two divergence classes: a label edited in
// the package but not yet applied (pending apply, package≠state) and a port
// changed in the cluster (cluster drift, state≠running).
func TestThreeWayClassifies(t *testing.T) {
	res, err := threeWay(svcPkg, svcSt, svcRun)
	if nil != err {
		t.Fatalf("threeWay: %v", err)
	}
	if 1 != len(res) {
		t.Fatalf("expected 1 resource, got %d", len(res))
	}
	out := formatThreeWay(res, false)

	// The label change is pending apply: package=new, state=old, running=old.
	if !strings.Contains(out, "metadata.labels.tier") || !strings.Contains(out, "pending apply") {
		t.Fatalf("expected pending-apply on label:\n%s", out)
	}
	// The port change is cluster drift: state=8080, running=9090.
	if !strings.Contains(out, "spec.ports.0.port") || !strings.Contains(out, "cluster drift") {
		t.Fatalf("expected cluster-drift on port:\n%s", out)
	}
	// All three columns must be shown for a differing field.
	if !strings.Contains(out, "package:") || !strings.Contains(out, "state:") || !strings.Contains(out, "running:") {
		t.Fatalf("expected three columns:\n%s", out)
	}
	if !strings.Contains(out, "1 cluster-drift, 1 pending-apply") {
		t.Fatalf("expected summary counts:\n%s", out)
	}
}

// TestThreeWayInSync skips resources where all three agree.
func TestThreeWayInSync(t *testing.T) {
	res, err := threeWay(svcSt, svcSt, svcSt)
	if nil != err {
		t.Fatalf("threeWay: %v", err)
	}
	if 0 != len(res) {
		t.Fatalf("expected no divergence, got %d resources", len(res))
	}
}

// TestThreeWayMissingFromCluster flags a resource in state but not running.
func TestThreeWayMissingFromCluster(t *testing.T) {
	res, err := threeWay(svcSt, svcSt, "")
	if nil != err {
		t.Fatalf("threeWay: %v", err)
	}
	if 1 != len(res) {
		t.Fatalf("expected 1 resource, got %d", len(res))
	}
	out := formatThreeWay(res, false)
	if !strings.Contains(out, "missing from cluster") {
		t.Fatalf("expected missing-from-cluster:\n%s", out)
	}
}
