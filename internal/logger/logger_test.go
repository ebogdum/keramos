package logger

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	fn()

	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestInitAndIsFlags(t *testing.T) {
	Init(false, false)
	if IsVerbose() {
		t.Fatal("expected verbose=false")
	}
	if IsDebug() {
		t.Fatal("expected debug=false")
	}

	Init(true, true)
	if !IsVerbose() {
		t.Fatal("expected verbose=true")
	}
	if !IsDebug() {
		t.Fatal("expected debug=true")
	}
}

func TestLogVerbose(t *testing.T) {
	Init(true, false)
	out := captureStderr(func() {
		Log("hello %s", "world")
	})
	if !strings.Contains(out, "hello world") {
		t.Fatalf("expected log output, got: %q", out)
	}
}

func TestLogSilent(t *testing.T) {
	Init(false, false)
	out := captureStderr(func() {
		Log("should not appear")
	})
	if "" != out {
		t.Fatalf("expected no output, got: %q", out)
	}
}

func TestDebugEnabled(t *testing.T) {
	Init(false, true)
	out := captureStderr(func() {
		Debug("trace %d", 42)
	})
	if !strings.Contains(out, "[DEBUG] trace 42") {
		t.Fatalf("expected debug output, got: %q", out)
	}
}

func TestDebugDisabled(t *testing.T) {
	Init(false, false)
	out := captureStderr(func() {
		Debug("should not appear")
	})
	if "" != out {
		t.Fatalf("expected no output, got: %q", out)
	}
}

func TestWarnAlwaysShown(t *testing.T) {
	Init(false, false)
	out := captureStderr(func() {
		Warn("caution %s", "here")
	})
	if !strings.Contains(out, "[WARN] caution here") {
		t.Fatalf("expected warn output, got: %q", out)
	}
}

func TestErrorAlwaysShown(t *testing.T) {
	Init(false, false)
	out := captureStderr(func() {
		Error("failure %s", "now")
	})
	if !strings.Contains(out, "[ERROR] failure now") {
		t.Fatalf("expected error output, got: %q", out)
	}
}

func TestLogWithDebugEnabled(t *testing.T) {
	Init(false, true)
	out := captureStderr(func() {
		Log("visible via debug")
	})
	if !strings.Contains(out, "visible via debug") {
		t.Fatalf("Log should output when debug=true, got: %q", out)
	}
}
