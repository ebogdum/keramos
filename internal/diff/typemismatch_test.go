package diff

import "testing"

// TestTypeMismatchReportsBothSides proves a map→scalar (or list→map) field
// change reports the new value instead of silently dropping it.
func TestTypeMismatchReportsBothSides(t *testing.T) {
	old := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  foo:\n    a: 1\n"
	new := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\ndata:\n  foo: bar\n"
	changes, err := Compute(old, new, Filters{})
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	found := false
	for _, f := range changes[0].FieldDiff {
		if f.Path == "data.foo" && f.New == "bar" {
			found = true
		}
	}
	if !found {
		t.Fatalf("map→scalar change dropped the new value: %+v", changes[0].FieldDiff)
	}
}
