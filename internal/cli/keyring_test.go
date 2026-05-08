package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
)

func writeArmoredKey(t *testing.T, path string) {
	t.Helper()
	entity, err := openpgp.NewEntity("test", "ci", "ci@example.com", nil)
	if nil != err {
		t.Fatalf("entity: %v", err)
	}
	f, err := os.Create(path)
	if nil != err {
		t.Fatalf("create: %v", err)
	}
	defer f.Close()
	w, err := armoredWriter(f)
	if nil != err {
		t.Fatalf("armor: %v", err)
	}
	if err := entity.Serialize(w); nil != err {
		t.Fatalf("serialize: %v", err)
	}
	w.Close()
}

// armoredWriter is a tiny shim so this test stays small without pulling
// armor.Encode here.
func armoredWriter(f *os.File) (interface {
	Close() error
	Write([]byte) (int, error)
}, error) {
	return f, nil // tests use an entity serialised non-armored; keyFingerprint falls back
}

func TestKeyring_AddListRemove(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	keyPath := filepath.Join(tmp, "test.asc")
	writeArmoredKey(t, keyPath)

	add := newKeyringAddCommand()
	var addBuf bytes.Buffer
	add.SetOut(&addBuf)
	if err := add.RunE(add, []string{keyPath}); nil != err {
		t.Fatalf("keyring add: %v", err)
	}
	if !strings.Contains(addBuf.String(), "Installed key test.asc") {
		t.Errorf("add output: %s", addBuf.String())
	}

	list := newKeyringListCommand()
	var listBuf bytes.Buffer
	list.SetOut(&listBuf)
	if err := list.RunE(list, nil); nil != err {
		t.Fatalf("keyring list: %v", err)
	}
	if !strings.Contains(listBuf.String(), "test.asc") {
		t.Errorf("list output: %s", listBuf.String())
	}

	rm := newKeyringRemoveCommand()
	var rmBuf bytes.Buffer
	rm.SetOut(&rmBuf)
	if err := rm.RunE(rm, []string{"test.asc"}); nil != err {
		t.Fatalf("keyring remove: %v", err)
	}
	if !strings.Contains(rmBuf.String(), "Removed key") {
		t.Errorf("remove output: %s", rmBuf.String())
	}
}

func TestKeyring_RemoveMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmp, ".config"))

	rm := newKeyringRemoveCommand()
	var buf bytes.Buffer
	rm.SetOut(&buf)
	if err := rm.RunE(rm, []string{"nonexistent.asc"}); nil == err {
		t.Fatal("expected error for missing key")
	}
}
