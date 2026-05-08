package repo

import (
	"strings"
	"testing"
)

func TestOCIPush_ArchiveNotFound(t *testing.T) {
	oci := &OCIRegistry{}
	err := oci.Push("/nonexistent/path/archive.keramos.tgz", "localhost:5000/test:v1")
	if nil == err {
		t.Fatal("expected error for nonexistent archive")
	}
	if !strings.Contains(err.Error(), "archive not found") {
		t.Errorf("expected error to mention 'archive not found', got: %v", err)
	}
}
