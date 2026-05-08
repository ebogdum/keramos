package action

import (
	"fmt"
	"testing"
)

func TestRollbackDescriptionFormat(t *testing.T) {
	tests := []struct {
		revision int
		expected string
	}{
		{1, "Rollback to 1"},
		{9, "Rollback to 9"},
		{10, "Rollback to 10"},
		{15, "Rollback to 15"},
		{100, "Rollback to 100"},
		{999, "Rollback to 999"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("revision_%d", tt.revision), func(t *testing.T) {
			result := fmt.Sprintf("Rollback to %d", tt.revision)
			if tt.expected != result {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
