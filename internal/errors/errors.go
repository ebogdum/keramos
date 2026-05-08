package errors

import (
	"fmt"
	"strings"
)

// ErrorType categorizes keramos errors for structured handling.
type ErrorType string

const (
	ErrParse            ErrorType = "PARSE"
	ErrExpression       ErrorType = "EXPRESSION"
	ErrIncludeCycle     ErrorType = "INCLUDE_CYCLE"
	ErrIncludeNotFound  ErrorType = "INCLUDE_NOT_FOUND"
	ErrUndefinedVar     ErrorType = "UNDEFINED_VAR"
	ErrFunction         ErrorType = "FUNCTION_ERROR"
	ErrType             ErrorType = "TYPE_ERROR"
	ErrSchemaValidation ErrorType = "SCHEMA_VALIDATION"
	ErrKube             ErrorType = "KUBE_ERROR"
	ErrRelease          ErrorType = "RELEASE_ERROR"
	ErrPackageInvalid   ErrorType = "PACKAGE_INVALID"
	ErrDependency       ErrorType = "DEPENDENCY_ERROR"
	ErrCLIFlag          ErrorType = "CLI_FLAG"
	ErrCLIValidation    ErrorType = "CLI_VALIDATION"
	ErrRepo             ErrorType = "REPO_ERROR"
	ErrArchive          ErrorType = "ARCHIVE_ERROR"
	ErrRegistry         ErrorType = "REGISTRY_ERROR"
	ErrAuth             ErrorType = "AUTH_ERROR"
	ErrConflict         ErrorType = "DEPENDENCY_CONFLICT"
	ErrCycle            ErrorType = "DEPENDENCY_CYCLE"
	ErrDigest           ErrorType = "DIGEST_MISMATCH"
	ErrSignature        ErrorType = "SIGNATURE_ERROR"
	ErrLockFile         ErrorType = "LOCKFILE_ERROR"
	ErrRateLimit        ErrorType = "RATE_LIMIT"
	ErrReleaseNotFound  ErrorType = "RELEASE_NOT_FOUND"
	ErrInternal         ErrorType = "INTERNAL"
)

// KeramosError is the structured error type used throughout keramos.
type KeramosError struct {
	Type       ErrorType
	Message    string
	FilePath   string
	Line       int
	Column     int
	Expression string
	Cause      error
	Context    map[string]string
}

// Error implements the error interface.
func (e *KeramosError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s", e.Type, e.Message)

	if e.FilePath != "" {
		fmt.Fprintf(&b, " (file: %s", e.FilePath)
		if e.Line > 0 {
			fmt.Fprintf(&b, ":%d", e.Line)
			if e.Column > 0 {
				fmt.Fprintf(&b, ":%d", e.Column)
			}
		}
		b.WriteString(")")
	}

	if e.Expression != "" {
		fmt.Fprintf(&b, " expr: %q", e.Expression)
	}

	if e.Cause != nil {
		fmt.Fprintf(&b, ": %s", e.Cause.Error())
	}

	return b.String()
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *KeramosError) Unwrap() error {
	return e.Cause
}

// NewError creates a KeramosError with a type and message.
func NewError(errType ErrorType, message string) *KeramosError {
	return &KeramosError{
		Type:    errType,
		Message: message,
	}
}

// NewErrorf creates a KeramosError with a formatted message.
func NewErrorf(errType ErrorType, format string, args ...any) *KeramosError {
	return &KeramosError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
	}
}

// WrapError wraps an existing error into a KeramosError.
func WrapError(errType ErrorType, message string, cause error) *KeramosError {
	return &KeramosError{
		Type:    errType,
		Message: message,
		Cause:   cause,
	}
}

// WrapErrorf wraps an existing error with a formatted message.
func WrapErrorf(errType ErrorType, cause error, format string, args ...any) *KeramosError {
	return &KeramosError{
		Type:    errType,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}

// WithFile attaches file location info and returns the same error for chaining.
func (e *KeramosError) WithFile(path string, line, column int) *KeramosError {
	e.FilePath = path
	e.Line = line
	e.Column = column
	return e
}

// WithExpression attaches an expression string and returns the same error.
func (e *KeramosError) WithExpression(expr string) *KeramosError {
	e.Expression = expr
	return e
}

// WithContext attaches a key-value pair to the context map and returns the same error.
func (e *KeramosError) WithContext(key, value string) *KeramosError {
	if nil == e.Context {
		e.Context = make(map[string]string)
	}
	e.Context[key] = value
	return e
}

// --- Domain-specific constructors ---

// ParseError creates a parse error with file location.
func ParseError(message, filePath string, line, column int) *KeramosError {
	return NewError(ErrParse, message).WithFile(filePath, line, column)
}

// ExpressionError creates an expression evaluation error.
func ExpressionError(message, expr string, cause error) *KeramosError {
	he := WrapError(ErrExpression, message, cause)
	he.Expression = expr
	return he
}

// PackageError creates a package-related error.
func PackageError(message, filePath string, cause error) *KeramosError {
	he := WrapError(ErrPackageInvalid, message, cause)
	he.FilePath = filePath
	return he
}

// KubeError creates a Kubernetes-related error.
func KubeError(message string, cause error) *KeramosError {
	return WrapError(ErrKube, message, cause)
}

// CLIError creates a CLI flag or validation error.
func CLIError(errType ErrorType, message string) *KeramosError {
	return NewError(errType, message)
}

// InternalError creates an internal/unexpected error.
func InternalError(message string, cause error) *KeramosError {
	return WrapError(ErrInternal, message, cause)
}

// FormatUserFriendly returns a human-readable error suitable for CLI output.
func FormatUserFriendly(err error) string {
	he, ok := err.(*KeramosError)
	if !ok {
		return fmt.Sprintf("Error: %s", err.Error())
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Error: %s\n", he.Message)

	if he.FilePath != "" {
		fmt.Fprintf(&b, "  File: %s", he.FilePath)
		if he.Line > 0 {
			fmt.Fprintf(&b, ", line %d", he.Line)
			if he.Column > 0 {
				fmt.Fprintf(&b, ", column %d", he.Column)
			}
		}
		b.WriteString("\n")
	}

	if he.Expression != "" {
		fmt.Fprintf(&b, "  Expression: %s\n", he.Expression)
	}

	if he.Cause != nil {
		fmt.Fprintf(&b, "  Caused by: %s\n", he.Cause.Error())
	}

	for k, v := range he.Context {
		fmt.Fprintf(&b, "  %s: %s\n", k, v)
	}

	return b.String()
}
