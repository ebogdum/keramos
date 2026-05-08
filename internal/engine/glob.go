package engine

import (
	"encoding/base64"
	"path/filepath"
	"strings"
)

// matchGlob matches a path against a glob pattern using path/filepath
// semantics. Supports `**` as a recursive segment matcher (one or more path
// components). Returns (matched, err).
func matchGlob(pattern, name string) (bool, error) {
	if "" == pattern || "*" == pattern {
		return true, nil
	}
	if strings.Contains(pattern, "**") {
		parts := strings.SplitN(pattern, "**", 2)
		prefix := strings.TrimRight(parts[0], "/")
		suffix := strings.TrimLeft(parts[1], "/")
		if "" != prefix && !strings.HasPrefix(name, prefix) {
			return false, nil
		}
		if "" == suffix {
			return true, nil
		}
		idx := strings.LastIndex(name, "/")
		tail := name
		if -1 != idx {
			tail = name[idx+1:]
		}
		return filepath.Match(suffix, tail)
	}
	return filepath.Match(pattern, name)
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
