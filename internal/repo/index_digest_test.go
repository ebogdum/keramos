package repo

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serveIndexWithDigest(t *testing.T, digest string) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "/index.yaml" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`apiVersion: v1
entries:
  myapp:
    - name: myapp
      version: "1.0.0"
      digest: ` + digest + `
      urls:
        - http://` + r.Host + `/packages/myapp-1.0.0.keramos.tgz
`))
			return
		}
		if "/packages/myapp-1.0.0.keramos.tgz" == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("fake-archive-content"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestDownloadRejectsDigestTheIndexDoesNotMatch(t *testing.T) {
	srv := serveIndexWithDigest(t, "0000000000000000000000000000000000000000000000000000000000000000")
	defer srv.Close()

	client := newTestRepoClient(t, &CredentialStore{Credentials: map[string]Credential{}})

	_, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil == err {
		t.Fatal("expected a download whose bytes do not match the index digest to fail")
	}
	if !strings.Contains(err.Error(), "digest") {
		t.Fatalf("expected the error to name the digest mismatch, got %v", err)
	}
}

func TestDownloadAcceptsDigestTheIndexMatches(t *testing.T) {
	srv := serveIndexWithDigest(t, "fd3d4b42292957ad0b649621615962140c857fbf7342038d6cc6b2b1ab8c3411")
	defer srv.Close()

	client := newTestRepoClient(t, &CredentialStore{Credentials: map[string]Credential{}})

	path, err := DownloadPackageWith(client, srv.URL, "myapp", "1.0.0")
	if nil != err {
		t.Fatalf("download of an archive matching its index digest failed: %v", err)
	}
	if "" == path {
		t.Fatal("expected a path to the downloaded archive")
	}
}
