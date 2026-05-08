package cli

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_NoDifference(t *testing.T) {
	a := "line1\nline2\nline3\n"
	b := "line1\nline2\nline3\n"

	result := UnifiedDiff(a, b, "a", "b")
	if "" != result {
		t.Fatalf("expected empty diff for identical input, got:\n%s", result)
	}
}

func TestUnifiedDiff_AddedLines(t *testing.T) {
	a := "line1\nline2\n"
	b := "line1\nline2\nline3\n"

	result := UnifiedDiff(a, b, "a", "b")
	if "" == result {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "+line3") {
		t.Fatalf("expected added line in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "--- a") {
		t.Fatalf("expected diff header, got:\n%s", result)
	}
	if !strings.Contains(result, "+++ b") {
		t.Fatalf("expected diff header, got:\n%s", result)
	}
}

func TestUnifiedDiff_RemovedLines(t *testing.T) {
	a := "line1\nline2\nline3\n"
	b := "line1\nline2\n"

	result := UnifiedDiff(a, b, "a", "b")
	if "" == result {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "-line3") {
		t.Fatalf("expected removed line in diff, got:\n%s", result)
	}
}

func TestUnifiedDiff_ChangedLines(t *testing.T) {
	a := "line1\nold-value\nline3\n"
	b := "line1\nnew-value\nline3\n"

	result := UnifiedDiff(a, b, "a", "b")
	if "" == result {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "-old-value") {
		t.Fatalf("expected old value removal in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+new-value") {
		t.Fatalf("expected new value addition in diff, got:\n%s", result)
	}
}

func TestUnifiedDiff_EmptyInputs(t *testing.T) {
	result := UnifiedDiff("", "", "a", "b")
	if "" != result {
		t.Fatalf("expected empty diff for empty inputs, got:\n%s", result)
	}
}

func TestUnifiedDiff_EmptyToContent(t *testing.T) {
	a := ""
	b := "line1\nline2\n"

	result := UnifiedDiff(a, b, "a", "b")
	if "" == result {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(result, "+line1") {
		t.Fatalf("expected added lines, got:\n%s", result)
	}
}

func TestUnifiedDiff_ManifestStyle(t *testing.T) {
	a := `apiVersion: v1
kind: ConfigMap
metadata:
  name: myconfig
data:
  key1: value1
  key2: old-value
`
	b := `apiVersion: v1
kind: ConfigMap
metadata:
  name: myconfig
data:
  key1: value1
  key2: new-value
  key3: added
`
	result := UnifiedDiff(a, b, "current", "proposed")
	if "" == result {
		t.Fatal("expected non-empty diff for manifest changes")
	}
	if !strings.Contains(result, "-  key2: old-value") {
		t.Fatalf("expected old value in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+  key2: new-value") {
		t.Fatalf("expected new value in diff, got:\n%s", result)
	}
	if !strings.Contains(result, "+  key3: added") {
		t.Fatalf("expected added key in diff, got:\n%s", result)
	}
}

func TestColorizeDiff_Colors(t *testing.T) {
	diff := "--- a\n+++ b\n@@ -1,2 +1,2 @@\n-old\n+new\n context\n"

	result := ColorizeDiff(diff)
	if !strings.Contains(result, "\033[31m") {
		t.Fatal("expected red color code for removed line")
	}
	if !strings.Contains(result, "\033[32m") {
		t.Fatal("expected green color code for added line")
	}
	if !strings.Contains(result, "\033[36m") {
		t.Fatal("expected cyan color code for hunk header")
	}
	if !strings.Contains(result, "\033[0m") {
		t.Fatal("expected color reset code")
	}
}

func TestColorizeDiff_ContextLinesUncolored(t *testing.T) {
	diff := " context-line\n"
	result := ColorizeDiff(diff)

	if strings.Contains(result, "\033[") {
		t.Fatal("context lines should not be colored")
	}
	if !strings.Contains(result, "context-line") {
		t.Fatal("context line content should be preserved")
	}
}

func TestSplitLines_TrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc\n")
	if 3 != len(result) {
		t.Fatalf("expected 3 lines, got %d: %v", len(result), result)
	}
}

func TestSplitLines_NoTrailingNewline(t *testing.T) {
	result := splitLines("a\nb\nc")
	if 3 != len(result) {
		t.Fatalf("expected 3 lines, got %d: %v", len(result), result)
	}
}

func TestSplitLines_Empty(t *testing.T) {
	result := splitLines("")
	if nil != result {
		t.Fatalf("expected nil for empty input, got %v", result)
	}
}
