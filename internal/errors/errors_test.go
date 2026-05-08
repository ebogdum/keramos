package errors

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestNewError(t *testing.T) {
	he := NewError(ErrParse, "unexpected token")
	if ErrParse != he.Type {
		t.Fatalf("expected type %s, got %s", ErrParse, he.Type)
	}
	if "unexpected token" != he.Message {
		t.Fatalf("expected message 'unexpected token', got %q", he.Message)
	}
}

func TestNewErrorf(t *testing.T) {
	he := NewErrorf(ErrType, "expected %s got %s", "int", "string")
	if !strings.Contains(he.Message, "expected int got string") {
		t.Fatalf("unexpected message: %s", he.Message)
	}
}

func TestWrapError(t *testing.T) {
	cause := fmt.Errorf("file not found")
	he := WrapError(ErrIncludeNotFound, "include failed", cause)
	if nil == he.Cause {
		t.Fatal("expected cause to be set")
	}
	if !errors.Is(he, cause) {
		t.Fatal("errors.Is should find the wrapped cause")
	}
}

func TestWrapErrorf(t *testing.T) {
	cause := fmt.Errorf("connection refused")
	he := WrapErrorf(ErrKube, cause, "kubectl failed for %s", "pods")
	if !strings.Contains(he.Message, "kubectl failed for pods") {
		t.Fatalf("unexpected message: %s", he.Message)
	}
	if nil == he.Cause {
		t.Fatal("expected cause to be set")
	}
}

func TestWithFile(t *testing.T) {
	he := NewError(ErrParse, "bad syntax").WithFile("values.yaml", 10, 5)
	if "values.yaml" != he.FilePath {
		t.Fatalf("expected FilePath 'values.yaml', got %q", he.FilePath)
	}
	if 10 != he.Line {
		t.Fatalf("expected line 10, got %d", he.Line)
	}
	if 5 != he.Column {
		t.Fatalf("expected column 5, got %d", he.Column)
	}
}

func TestWithExpression(t *testing.T) {
	he := NewError(ErrExpression, "eval failed").WithExpression("{{ .Values.x }}")
	if "{{ .Values.x }}" != he.Expression {
		t.Fatalf("expected expression '{{ .Values.x }}', got %q", he.Expression)
	}
}

func TestWithContext(t *testing.T) {
	he := NewError(ErrInternal, "oops").WithContext("key1", "val1").WithContext("key2", "val2")
	if 2 != len(he.Context) {
		t.Fatalf("expected 2 context entries, got %d", len(he.Context))
	}
	if "val1" != he.Context["key1"] {
		t.Fatalf("expected context key1=val1, got %q", he.Context["key1"])
	}
}

func TestErrorString(t *testing.T) {
	he := NewError(ErrParse, "bad token").WithFile("f.yaml", 3, 7).WithExpression("$x")
	s := he.Error()
	if !strings.Contains(s, "[PARSE]") {
		t.Fatal("error string should contain type")
	}
	if !strings.Contains(s, "f.yaml:3:7") {
		t.Fatal("error string should contain file location")
	}
	if !strings.Contains(s, `"$x"`) {
		t.Fatal("error string should contain expression")
	}
}

func TestErrorStringNoCause(t *testing.T) {
	he := NewError(ErrInternal, "boom")
	s := he.Error()
	if strings.Contains(s, ":") && strings.Count(s, ":") > 1 {
		// Just ensure no crash
	}
	if !strings.Contains(s, "boom") {
		t.Fatal("should contain message")
	}
}

func TestErrorStringWithCause(t *testing.T) {
	cause := fmt.Errorf("underlying")
	he := WrapError(ErrKube, "k8s problem", cause)
	s := he.Error()
	if !strings.Contains(s, "underlying") {
		t.Fatal("should contain cause message")
	}
}

func TestParseError(t *testing.T) {
	he := ParseError("bad yaml", "keramos.yaml", 5, 2)
	if ErrParse != he.Type {
		t.Fatalf("expected PARSE, got %s", he.Type)
	}
	if "keramos.yaml" != he.FilePath {
		t.Fatalf("expected keramos.yaml, got %s", he.FilePath)
	}
}

func TestExpressionError(t *testing.T) {
	cause := fmt.Errorf("divide by zero")
	he := ExpressionError("eval failed", "1/0", cause)
	if ErrExpression != he.Type {
		t.Fatalf("expected EXPRESSION, got %s", he.Type)
	}
	if "1/0" != he.Expression {
		t.Fatalf("expected expression '1/0', got %q", he.Expression)
	}
}

func TestPackageError(t *testing.T) {
	cause := fmt.Errorf("no such file")
	he := PackageError("load failed", "/tmp/keramos.yaml", cause)
	if ErrPackageInvalid != he.Type {
		t.Fatalf("expected PACKAGE_INVALID, got %s", he.Type)
	}
}

func TestKubeError(t *testing.T) {
	cause := fmt.Errorf("timeout")
	he := KubeError("apply failed", cause)
	if ErrKube != he.Type {
		t.Fatalf("expected KUBE_ERROR, got %s", he.Type)
	}
}

func TestCLIError(t *testing.T) {
	he := CLIError(ErrCLIFlag, "unknown flag --foo")
	if ErrCLIFlag != he.Type {
		t.Fatalf("expected CLI_FLAG, got %s", he.Type)
	}
}

func TestInternalError(t *testing.T) {
	cause := fmt.Errorf("nil pointer")
	he := InternalError("unexpected", cause)
	if ErrInternal != he.Type {
		t.Fatalf("expected INTERNAL, got %s", he.Type)
	}
}

func TestFormatUserFriendlyKeramosError(t *testing.T) {
	cause := fmt.Errorf("EOF")
	he := WrapError(ErrParse, "parse failed", cause).
		WithFile("x.yaml", 1, 2).
		WithExpression("{{ bad }}").
		WithContext("hint", "check syntax")

	out := FormatUserFriendly(he)
	if !strings.Contains(out, "Error: parse failed") {
		t.Fatal("should contain message")
	}
	if !strings.Contains(out, "File: x.yaml") {
		t.Fatal("should contain file")
	}
	if !strings.Contains(out, "line 1") {
		t.Fatal("should contain line")
	}
	if !strings.Contains(out, "column 2") {
		t.Fatal("should contain column")
	}
	if !strings.Contains(out, "Expression: {{ bad }}") {
		t.Fatal("should contain expression")
	}
	if !strings.Contains(out, "Caused by: EOF") {
		t.Fatal("should contain cause")
	}
	if !strings.Contains(out, "hint: check syntax") {
		t.Fatal("should contain context")
	}
}

func TestFormatUserFriendlyGenericError(t *testing.T) {
	err := fmt.Errorf("generic problem")
	out := FormatUserFriendly(err)
	if !strings.Contains(out, "Error: generic problem") {
		t.Fatal("should format generic errors")
	}
}

func TestUnwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	he := WrapError(ErrInternal, "wrapper", cause)
	unwrapped := errors.Unwrap(he)
	if cause != unwrapped {
		t.Fatal("Unwrap should return cause")
	}
}

func TestErrorFileWithoutLineColumn(t *testing.T) {
	he := NewError(ErrParse, "oops").WithFile("a.yaml", 0, 0)
	s := he.Error()
	if !strings.Contains(s, "(file: a.yaml)") {
		t.Fatalf("should show file without line/column: %s", s)
	}
}

func TestErrorFileWithLineNoColumn(t *testing.T) {
	he := NewError(ErrParse, "oops").WithFile("a.yaml", 5, 0)
	s := he.Error()
	if !strings.Contains(s, "a.yaml:5)") {
		t.Fatalf("should show file:line without column: %s", s)
	}
}
