package action

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSchema(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "values.schema.json"), []byte(content), 0644); nil != err {
		t.Fatalf("failed to write schema: %v", err)
	}
}

func TestSchema_AllOf(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"allOf": [
			{"type": "object", "required": ["a"]},
			{"type": "object", "required": ["b"]}
		]
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"a": 1}); nil == err || !strings.Contains(err.Error(), "b") {
		t.Fatalf("expected allOf to enforce both required, got: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"a": 1, "b": 2}); nil != err {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchema_AnyOfOneOf(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {
			"x": {"anyOf": [{"type": "integer"}, {"type": "string"}]},
			"y": {"oneOf": [{"type": "integer"}, {"type": "string"}]}
		}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x": "ok", "y": 42}); nil != err {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x": true}); nil == err {
		t.Fatal("expected anyOf failure for bool")
	}
}

func TestSchema_Pattern(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {"name": {"type": "string", "pattern": "^[a-z]+$"}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"name": "abc"}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"name": "ABC"}); nil == err {
		t.Fatal("expected pattern mismatch")
	}
}

func TestSchema_Format(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {"email": {"type": "string", "format": "email"}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"email": "a@b.com"}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"email": "not-an-email"}); nil == err {
		t.Fatal("expected format failure")
	}
}

func TestSchema_RefAndDefs(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"$defs": {"port": {"type": "integer", "minimum": 1, "maximum": 65535}},
		"properties": {"port": {"$ref": "#/$defs/port"}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"port": 8080}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"port": 70000}); nil == err {
		t.Fatal("expected port out-of-range")
	}
}

func TestSchema_Const(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {"kind": {"const": "Service"}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"kind": "Service"}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"kind": "Pod"}); nil == err {
		t.Fatal("expected const mismatch")
	}
}

func TestSchema_DependentRequired(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"dependentRequired": {"creditCard": ["billingAddress"]}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"creditCard": "x"}); nil == err {
		t.Fatal("expected dependentRequired error")
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{}); nil != err {
		t.Fatalf("trigger missing should pass: %v", err)
	}
}

func TestSchema_UniqueItems(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {"tags": {"type": "array", "uniqueItems": true}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"tags": []any{"a", "b"}}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"tags": []any{"a", "a"}}); nil == err {
		t.Fatal("expected duplicate detection")
	}
}

func TestSchema_MultipleOf(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{"properties": {"n": {"type": "number", "multipleOf": 0.5}}}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"n": 1.5}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"n": 1.7}); nil == err {
		t.Fatal("expected multipleOf failure")
	}
}

func TestSchema_PatternProperties(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"patternProperties": {"^x_": {"type": "string"}},
		"additionalProperties": false
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x_a": "ok"}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"y": "no"}); nil == err {
		t.Fatal("expected additionalProperties false rejection")
	}
}

func TestSchema_Not(t *testing.T) {
	dir := t.TempDir()
	writeSchema(t, dir, `{
		"properties": {"x": {"not": {"type": "string"}}}
	}`)
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x": 42}); nil != err {
		t.Fatalf("unexpected: %v", err)
	}
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x": "string"}); nil == err {
		t.Fatal("expected not-schema rejection")
	}
}

func TestSchema_NoFile(t *testing.T) {
	dir := t.TempDir()
	if err := ValidateValuesAgainstSchema(dir, map[string]any{"x": 1}); nil != err {
		t.Fatalf("missing schema should pass: %v", err)
	}
}
