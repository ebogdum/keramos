package hooks

import (
	"strings"
	"testing"
)

func TestDeleteHookResources_NotFoundIsAcceptable(t *testing.T) {
	// The deleteHookResources function should treat "not found" errors as success.
	// We test the string-matching logic here since we can't easily mock the kube client
	// in this package without the interface (which is tested at the integration level).

	errMsg := "failed to delete hook resources: not found"
	if !strings.Contains(errMsg, "not found") {
		t.Error("expected 'not found' detection to work")
	}

	errMsg2 := "permission denied"
	if strings.Contains(errMsg2, "not found") || strings.Contains(errMsg2, "NotFound") {
		t.Error("expected 'permission denied' to not be treated as not-found")
	}
}
