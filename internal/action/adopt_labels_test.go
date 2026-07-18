package action

import "testing"

// TestAdoptRecordsLabels proves the --labels flag reaches the stored release:
// AdoptOptions.Labels must be persisted on the adopted release record.
func TestAdoptRecordsLabels(t *testing.T) {
	client := newMockClient("apps")
	client.lookupFn = func(apiVersion, kind, namespace, name string) (map[string]any, error) {
		return map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata":   map[string]any{"name": name, "namespace": namespace},
			"data":       map[string]any{"k": "v"},
		}, nil
	}
	rel, err := Adopt(client, &AdoptOptions{
		ReleaseName: "adopted",
		Namespace:   "apps",
		Resources:   []ResourceRef{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "apps", Name: "cm"}},
		Labels:      map[string]string{"team": "platform"},
	})
	if nil != err {
		t.Fatalf("adopt: %v", err)
	}
	if "platform" != rel.Labels["team"] {
		t.Fatalf("labels not recorded on adopted release: %v", rel.Labels)
	}
}
