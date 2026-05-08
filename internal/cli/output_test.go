package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestFormatTable_Basic(t *testing.T) {
	headers := []string{"NAME", "AGE", "STATUS"}
	rows := [][]string{
		{"alice", "30", "active"},
		{"bob", "25", "inactive"},
	}

	result := FormatTable(headers, rows)

	if !strings.Contains(result, "NAME") {
		t.Fatal("expected header NAME in output")
	}
	if !strings.Contains(result, "alice") {
		t.Fatal("expected row value alice in output")
	}
	if !strings.Contains(result, "bob") {
		t.Fatal("expected row value bob in output")
	}

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if 3 != len(lines) {
		t.Fatalf("expected 3 lines (1 header + 2 rows), got %d", len(lines))
	}
}

func TestFormatTable_EmptyHeaders(t *testing.T) {
	result := FormatTable(nil, nil)
	if "" != result {
		t.Fatalf("expected empty string for nil headers, got %q", result)
	}
}

func TestFormatTable_EmptyRows(t *testing.T) {
	headers := []string{"COL1", "COL2"}
	result := FormatTable(headers, nil)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if 1 != len(lines) {
		t.Fatalf("expected 1 line (header only), got %d", len(lines))
	}
}

func TestFormatTable_ColumnAlignment(t *testing.T) {
	headers := []string{"A", "B"}
	rows := [][]string{
		{"short", "x"},
		{"a-much-longer-value", "y"},
	}

	result := FormatTable(headers, rows)
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")

	// All lines should have the same position for column B separator
	// The second column should start at the same position in all lines
	headerBPos := strings.Index(lines[0], "B")
	for i, line := range lines[1:] {
		// Find the start of the second column value
		parts := strings.Fields(line)
		if 2 != len(parts) {
			t.Fatalf("row %d: expected 2 fields, got %d", i, len(parts))
		}
		// The second value should appear after padding
		secondColPos := strings.LastIndex(line, parts[1])
		if secondColPos < headerBPos-1 {
			t.Fatalf("row %d: column B misaligned (header at %d, value at %d)", i, headerBPos, secondColPos)
		}
	}
}

func TestFormatJSON_ValidOutput(t *testing.T) {
	data := map[string]any{
		"name":    "test",
		"version": 1,
	}

	result, err := FormatJSON(data)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if jsonErr := json.Unmarshal([]byte(result), &parsed); nil != jsonErr {
		t.Fatalf("output is not valid JSON: %v", jsonErr)
	}

	if "test" != parsed["name"] {
		t.Fatalf("expected name=test, got %v", parsed["name"])
	}
}

func TestFormatJSON_Indented(t *testing.T) {
	data := map[string]string{"key": "value"}
	result, err := FormatJSON(data)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "  ") {
		t.Fatal("expected indented output")
	}
}

func TestFormatJSON_TrailingNewline(t *testing.T) {
	data := "hello"
	result, err := FormatJSON(data)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasSuffix(result, "\n") {
		t.Fatal("expected trailing newline")
	}
}

func TestFormatYAML_ValidOutput(t *testing.T) {
	data := map[string]any{
		"name":    "test",
		"version": 1,
	}

	result, err := FormatYAML(data)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if yamlErr := yaml.Unmarshal([]byte(result), &parsed); nil != yamlErr {
		t.Fatalf("output is not valid YAML: %v", yamlErr)
	}

	if "test" != parsed["name"] {
		t.Fatalf("expected name=test, got %v", parsed["name"])
	}
}

func TestFormatYAML_MapOutput(t *testing.T) {
	data := map[string]string{
		"app":     "myapp",
		"version": "1.0",
	}

	result, err := FormatYAML(data)
	if nil != err {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(result, "app: myapp") {
		t.Fatal("expected YAML key-value pair in output")
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abc", 1, "abc"},
		{"", 3, "   "},
	}

	for _, tt := range tests {
		result := padRight(tt.input, tt.width)
		if tt.expected != result {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, result, tt.expected)
		}
	}
}
