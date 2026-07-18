package values

import (
	"strings"
	"testing"
)

func TestTraceProvenance(t *testing.T) {
	_, trace, err := ResolveAllWithTrace(map[string]any{"replicas": 1}, nil, []string{"replicas=9"}, nil, nil, nil)
	if nil != err {
		t.Fatalf("resolve: %v", err)
	}
	p := trace.Provenance()
	if !strings.Contains(p["replicas"], "set") || !strings.Contains(p["replicas"], "replicas=9") {
		t.Fatalf("replicas provenance = %q", p["replicas"])
	}
}
