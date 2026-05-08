package logger

import (
	"fmt"
	"os"
	"sync/atomic"
)

var (
	verboseFlag atomic.Bool
	debugFlag   atomic.Bool
)

// Init configures the logger. Call once at startup.
func Init(v, d bool) {
	verboseFlag.Store(v)
	debugFlag.Store(d)
}

// Log prints an informational message to stderr when verbose or debug is enabled.
func Log(format string, args ...any) {
	if !verboseFlag.Load() && !debugFlag.Load() {
		return
	}
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

// Debug prints a debug message to stderr when debug mode is enabled.
func Debug(format string, args ...any) {
	if !debugFlag.Load() {
		return
	}
	fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
}

// Warn prints a warning message to stderr (always shown).
func Warn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", args...)
}

// Error prints an error message to stderr (always shown).
func Error(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[ERROR] "+format+"\n", args...)
}

// IsVerbose returns whether verbose logging is enabled.
func IsVerbose() bool {
	return verboseFlag.Load()
}

// IsDebug returns whether debug logging is enabled.
func IsDebug() bool {
	return debugFlag.Load()
}
