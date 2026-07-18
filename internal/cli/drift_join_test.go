package cli

import (
	"strings"
	"testing"
)

// TestThreeWayParsesMultiDocLive proves the three-way comparison correctly
// reads a live manifest of several resources (regression: a "---\n" join glued
// the separator to compact JSON and dropped every doc after the first).
func TestThreeWayParsesMultiDocLive(t *testing.T) {
	// Two compact-JSON docs joined the correct way (as action.collectLiveManifest does).
	live := strings.Join([]string{
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a","namespace":"x"},"data":{"k":"1"}}`,
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"b","namespace":"x"},"data":{"k":"2"}}`,
	}, "\n---\n")
	res, err := threeWay(live, live, live)
	if nil != err {
		t.Fatalf("threeWay: %v", err)
	}
	// identical -> no divergence, but the important part is both docs parsed
	if 0 != len(res) {
		t.Fatalf("identical inputs should show no divergence, got %d", len(res))
	}
	// Now change one field in the live side of resource b only.
	live2 := strings.Join([]string{
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a","namespace":"x"},"data":{"k":"1"}}`,
		`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"b","namespace":"x"},"data":{"k":"CHANGED"}}`,
	}, "\n---\n")
	res2, err := threeWay(live, live, live2)
	if nil != err {
		t.Fatalf("threeWay: %v", err)
	}
	found := false
	for _, r := range res2 {
		if "b" == r.name {
			found = true
		}
	}
	if !found {
		t.Fatal("second resource (b) was not parsed/compared — multi-doc join regression")
	}
}
