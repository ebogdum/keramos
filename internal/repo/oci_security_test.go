package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content/file"
)

// orasUnpackAnnotation mirrors the internal ORAS file-store annotation that
// requests tar unpacking of a pulled layer. A malicious registry controls this
// value, so keramos must not honour it.
const orasUnpackAnnotation = "io.deis.oras.content.unpack"

// TestPullFileStoreSkipsUntrustedUnpack proves the mitigation for the ORAS
// hardlink/symlink tar-extraction escape (no fixed oras-go release as of
// v2.6.1): with SkipUnpack set, a layer whose descriptor carries the untrusted
// unpack annotation and a crafted tar.gz containing a hardlink with a relative
// Linkname is written verbatim to the destination and NOT extracted, so no file
// escapes the destination directory.
func TestPullFileStoreSkipsUntrustedUnpack(t *testing.T) {
	destDir := t.TempDir()

	// Build a gzip-compressed tar carrying a hardlink whose Linkname points
	// outside the extraction root — the exact shape ORAS would extract.
	var raw bytes.Buffer
	gz := gzip.NewWriter(&raw)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "escape",
		Typeflag: tar.TypeLink,
		Linkname: "../../../../etc/keramos-escape-target",
	}); nil != err {
		t.Fatal(err)
	}
	if err := tw.Close(); nil != err {
		t.Fatal(err)
	}
	if err := gz.Close(); nil != err {
		t.Fatal(err)
	}
	payload := raw.Bytes()

	fs, err := file.New(destDir)
	if nil != err {
		t.Fatal(err)
	}
	defer fs.Close()

	// This is the guard exercised by OCIRegistry.Pull.
	fs.SkipUnpack = true

	desc := ocispec.Descriptor{
		MediaType: ocispec.MediaTypeImageLayerGzip,
		Digest:    digest.FromBytes(payload),
		Size:      int64(len(payload)),
		Annotations: map[string]string{
			orasUnpackAnnotation:    "true",
			ocispec.AnnotationTitle: "payload.tar.gz",
		},
	}

	if err := fs.Push(context.Background(), desc, bytes.NewReader(payload)); nil != err {
		t.Fatalf("push with SkipUnpack should store the layer as a file, got: %v", err)
	}

	// The blob must land as a single file, not an extracted tree.
	if _, err := os.Stat(filepath.Join(destDir, "payload.tar.gz")); nil != err {
		t.Fatalf("expected pulled layer stored verbatim as payload.tar.gz: %v", err)
	}

	// The hardlink escape target must never have been created.
	escape := filepath.Join(filepath.Dir(destDir), "..", "..", "..", "etc", "keramos-escape-target")
	if _, err := os.Lstat(filepath.Clean(escape)); !os.IsNotExist(err) {
		t.Fatalf("hardlink escape target was created outside destDir: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(destDir, "escape")); !os.IsNotExist(err) {
		t.Fatal("layer was unpacked despite SkipUnpack")
	}
}
