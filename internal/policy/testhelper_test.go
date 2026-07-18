package policy

import (
	"os"
	"path/filepath"
)

func mkdirWrite(dir, name, body string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644)
}
