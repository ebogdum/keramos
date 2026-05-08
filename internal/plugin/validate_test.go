package plugin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidateCommand_RejectsPathSeparators(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, "sub/run.sh")
	if nil == err {
		t.Fatal("expected error for command with path separator")
	}
}

func TestValidateCommand_RejectsParentTraversal(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, "..run.sh")
	if nil == err {
		t.Fatal("expected error for command with .. in name")
	}
}

func TestValidateCommand_RejectsNonexistent(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, "nonexistent.sh")
	if nil == err {
		t.Fatal("expected error for nonexistent command")
	}
}

func TestValidateCommand_RejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "subdir")
	if mkErr := os.Mkdir(subDir, 0o755); nil != mkErr {
		t.Fatal(mkErr)
	}

	err := validateCommand(dir, "subdir")
	if nil == err {
		t.Fatal("expected error for directory command")
	}
}

func TestValidateCommand_RejectsNonExecutable(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping executable check on Windows")
	}

	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o644); nil != err {
		t.Fatal(err)
	}

	err := validateCommand(dir, "run.sh")
	if nil == err {
		t.Fatal("expected error for non-executable command")
	}
}

func TestValidateCommand_AcceptsValid(t *testing.T) {
	if "windows" == runtime.GOOS {
		t.Skip("skipping executable check on Windows")
	}

	dir := t.TempDir()
	cmdPath := filepath.Join(dir, "run.sh")
	if err := os.WriteFile(cmdPath, []byte("#!/bin/sh\n"), 0o755); nil != err {
		t.Fatal(err)
	}

	err := validateCommand(dir, "run.sh")
	if nil != err {
		t.Fatalf("expected valid command to pass, got: %v", err)
	}
}

func TestValidateCommand_RejectsBackslashSeparator(t *testing.T) {
	dir := t.TempDir()
	err := validateCommand(dir, `sub\run.sh`)
	if nil == err {
		t.Fatal("expected error for command with backslash separator")
	}
}
