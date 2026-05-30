package action

import (
	"testing"
)

const driftDeployManifest = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: default
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.0
`

func TestDriftReportsMissingResource(t *testing.T) {
	mock := newMockClient("default")
	// default Lookup returns nil -> resource absent in cluster.
	items, err := DriftAgainstManifest(mock, driftDeployManifest)
	if nil != err {
		t.Fatalf("drift: %v", err)
	}
	if 1 != len(items) {
		t.Fatalf("expected 1 drift item, got %d", len(items))
	}
	if DriftMissing != items[0].Kind {
		t.Fatalf("expected DriftMissing, got %s", items[0].Kind)
	}
	if "web" != items[0].Name {
		t.Fatalf("unexpected name %q", items[0].Name)
	}
}

func TestDriftReportsMutatedSpec(t *testing.T) {
	mock := newMockClient("default")
	// Live object differs: replicas drifted 3 -> 1.
	mock.lookupFn = func(_, kind, _, _ string) (map[string]any, error) {
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": "web", "namespace": "default"},
			"spec": map[string]any{
				"replicas": int64(1),
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "web", "image": "nginx:1.0"},
						},
					},
				},
			},
		}, nil
	}
	items, err := DriftAgainstManifest(mock, driftDeployManifest)
	if nil != err {
		t.Fatalf("drift: %v", err)
	}
	if 1 != len(items) {
		t.Fatalf("expected 1 drift item, got %d", len(items))
	}
	if DriftMutated != items[0].Kind {
		t.Fatalf("expected DriftMutated, got %s", items[0].Kind)
	}
	foundReplicas := false
	for _, fd := range items[0].FieldDiffs {
		if "spec.replicas" == fd.Path {
			foundReplicas = true
		}
	}
	if !foundReplicas {
		t.Fatalf("expected a spec.replicas field diff, got %+v", items[0].FieldDiffs)
	}
}

func TestDriftNoneWhenLiveMatches(t *testing.T) {
	mock := newMockClient("default")
	mock.lookupFn = func(_, kind, _, _ string) (map[string]any, error) {
		return map[string]any{
			"apiVersion": "apps/v1",
			"kind":       kind,
			"metadata":   map[string]any{"name": "web", "namespace": "default"},
			"spec": map[string]any{
				"replicas": int64(3),
				"template": map[string]any{
					"spec": map[string]any{
						"containers": []any{
							map[string]any{"name": "web", "image": "nginx:1.0"},
						},
					},
				},
			},
		}, nil
	}
	items, err := DriftAgainstManifest(mock, driftDeployManifest)
	if nil != err {
		t.Fatalf("drift: %v", err)
	}
	if 0 != len(items) {
		t.Fatalf("expected no drift, got %+v", items)
	}
}
