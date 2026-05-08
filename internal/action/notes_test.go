package action

import (
	"strings"
	"testing"
)

func TestExtractNotes_WithNotesDoc(t *testing.T) {
	rendered := "apiVersion: v1\nkind: Service\nmetadata:\n  name: test\n---\nmessage: |\n  Hello from notes.\n"

	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "" == notes {
		t.Fatal("expected non-empty notes")
	}
	if !strings.Contains(notes, "Hello from notes.") {
		t.Errorf("expected notes to contain message, got %q", notes)
	}
	if strings.Contains(manifest, "message:") {
		t.Error("expected notes document to be removed from manifest")
	}
	if !strings.Contains(manifest, "kind: Service") {
		t.Error("expected service manifest to be preserved")
	}
}

func TestExtractNotes_NoNotesDoc(t *testing.T) {
	rendered := "apiVersion: v1\nkind: Service\nmetadata:\n  name: test\n"

	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "" != notes {
		t.Errorf("expected empty notes, got %q", notes)
	}
	if !strings.Contains(manifest, "kind: Service") {
		t.Error("expected manifest to be preserved")
	}
}

func TestExtractNotes_EmptyInput(t *testing.T) {
	manifest, notes, err := extractNotes("")
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
	if "" != manifest {
		t.Errorf("expected empty manifest, got %q", manifest)
	}
	if "" != notes {
		t.Errorf("expected empty notes, got %q", notes)
	}
}

func TestExtractNotes_MultipleManifests(t *testing.T) {
	rendered := "apiVersion: v1\nkind: Service\nmetadata:\n  name: svc\n---\nmessage: |\n  Install complete.\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: dep\n"

	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(notes, "Install complete.") {
		t.Errorf("expected notes message, got %q", notes)
	}
	if strings.Contains(manifest, "message:") {
		t.Error("notes document should be removed from manifest")
	}
	if !strings.Contains(manifest, "kind: Service") {
		t.Error("service should be preserved")
	}
	if !strings.Contains(manifest, "kind: Deployment") {
		t.Error("deployment should be preserved")
	}
}

func TestExtractNotes_MapWithMultipleKeys(t *testing.T) {
	rendered := "message: hello\nextra: key\n"

	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "" != notes {
		t.Errorf("expected empty notes for multi-key doc, got %q", notes)
	}
	if !strings.Contains(manifest, "message: hello") {
		t.Error("expected doc to remain in manifest")
	}
}

func TestExtractNotes_MessageNotString(t *testing.T) {
	rendered := "message:\n  nested: value\n"

	manifest, notes, err := extractNotes(rendered)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if "" != notes {
		t.Errorf("expected empty notes for non-string message, got %q", notes)
	}
	if "" == manifest {
		t.Error("expected manifest to be preserved")
	}
}
