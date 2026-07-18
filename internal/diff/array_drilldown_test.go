package diff

import "testing"

// TestArrayElementDrilldown verifies that a change to one field of one element
// in a same-length list is reported at its indexed path, not as a whole-list
// replacement — so a changed container image is attributable to its source.
func TestArrayElementDrilldown(t *testing.T) {
	base := "apiVersion: apps/v1\n" +
		"kind: Deployment\n" +
		"metadata:\n  name: app\n" +
		"spec:\n" +
		"  template:\n    spec:\n      containers:\n" +
		"        - name: app\n          image: nginx:1.24\n"
	target := "apiVersion: apps/v1\n" +
		"kind: Deployment\n" +
		"metadata:\n  name: app\n" +
		"spec:\n" +
		"  template:\n    spec:\n      containers:\n" +
		"        - name: app\n          image: nginx:1.25\n"

	changes, err := Compute(base, target, Filters{ShowDefaultedFields: true, ShowImagePullPolicy: true})
	if nil != err {
		t.Fatalf("compute: %v", err)
	}
	if 1 != len(changes) {
		t.Fatalf("expected 1 resource change, got %d", len(changes))
	}
	var found *FieldChange
	for i := range changes[0].FieldDiff {
		if changes[0].FieldDiff[i].Path == "spec.template.spec.containers.0.image" {
			found = &changes[0].FieldDiff[i]
		}
	}
	if nil == found {
		t.Fatalf("image change not drilled to indexed path; got fields: %+v", changes[0].FieldDiff)
	}
	if "nginx:1.24" != found.Old || "nginx:1.25" != found.New {
		t.Fatalf("wrong old/new: %v -> %v", found.Old, found.New)
	}
}

// TestArrayLengthChangeReportedWhole verifies that adding/removing an element
// (differing lengths) is still reported as a whole-list change.
func TestArrayLengthChangeReportedWhole(t *testing.T) {
	base := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\nspec:\n  containers:\n    - name: a\n"
	target := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: p\nspec:\n  containers:\n    - name: a\n    - name: b\n"
	changes, err := Compute(base, target, Filters{})
	if nil != err {
		t.Fatalf("compute: %v", err)
	}
	if 1 != len(changes) {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	for _, f := range changes[0].FieldDiff {
		if f.Path == "spec.containers" {
			return // reported whole, as expected
		}
	}
	t.Fatalf("expected whole-list change at spec.containers, got: %+v", changes[0].FieldDiff)
}
