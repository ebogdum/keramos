package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/repo"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newPublishCommand() *cobra.Command {
	var repoURL string
	var ociRef string

	cmd := &cobra.Command{
		Use:   "publish <archive.keramos.tgz>",
		Short: "Publish a package archive to a registry",
		Long:  "Publish a .keramos.tgz archive to an HTTP API registry (--repo) or an OCI registry (--oci).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			archivePath := args[0]
			return runPublish(cmd, archivePath, repoURL, ociRef)
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "HTTP API registry URL")
	cmd.Flags().StringVar(&ociRef, "oci", "", "OCI registry reference")

	return cmd
}

func runPublish(cmd *cobra.Command, archivePath, repoURL, ociRef string) error {
	if "" == repoURL && "" == ociRef {
		return keramoserr.NewError(keramoserr.ErrCLIFlag, "specify --repo or --oci")
	}

	absPath, err := filepath.Abs(archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to resolve archive path", err)
	}

	if !strings.HasSuffix(absPath, ".keramos.tgz") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "archive must be a .keramos.tgz file: %s", absPath)
	}

	info, err := os.Stat(absPath)
	if nil != err {
		return keramoserr.WrapErrorf(keramoserr.ErrArchive, err, "archive not found: %s", absPath)
	}
	if info.IsDir() {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "expected a file, got directory: %s", absPath)
	}

	meta, err := extractArchiveMetadata(absPath)
	if nil != err {
		return err
	}

	if "" != ociRef {
		return publishOCI(cmd, absPath, ociRef, meta)
	}

	return publishHTTP(cmd, absPath, repoURL, meta)
}

func extractArchiveMetadata(archivePath string) (*archiveMeta, error) {
	tmpDir, err := os.MkdirTemp("", "keramos-publish-*")
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrArchive, "failed to create temp directory", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := repo.ExtractArchive(archivePath, tmpDir); nil != err {
		return nil, err
	}

	entries, err := os.ReadDir(tmpDir)
	if nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrArchive, "failed to read extracted archive", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(tmpDir, entry.Name(), "keramos.yaml")
		if _, statErr := os.Stat(metaPath); nil != statErr {
			continue
		}

		// Read name and version from the keramos.yaml via the metadata loader
		pkgDir := filepath.Join(tmpDir, entry.Name())
		m, loadErr := loadMinimalMeta(pkgDir)
		if nil == loadErr {
			return m, nil
		}
	}

	return nil, keramoserr.NewError(keramoserr.ErrArchive, "archive does not contain a valid keramos.yaml")
}

type archiveMeta struct {
	Name    string
	Version string
}

func loadMinimalMeta(dir string) (*archiveMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "keramos.yaml"))
	if nil != err {
		return nil, err
	}

	// Parse just name and version without importing the full pkg loader
	// to avoid circular dependency issues. We use a simple map approach.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); nil != err {
		return nil, err
	}

	name, _ := raw["name"].(string)
	version, _ := raw["version"].(string)

	if "" == name || "" == version {
		return nil, keramoserr.NewError(keramoserr.ErrArchive, "keramos.yaml missing name or version")
	}

	return &archiveMeta{Name: name, Version: version}, nil
}

func publishOCI(cmd *cobra.Command, archivePath, ociRef string, meta *archiveMeta) error {
	ociReg := &repo.OCIRegistry{}
	if err := ociReg.Push(archivePath, ociRef); nil != err {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s to OCI registry %s\n", meta.Name, meta.Version, ociRef)
	return nil
}

func publishHTTP(cmd *cobra.Command, archivePath, repoURL string, meta *archiveMeta) error {
	client, err := repo.DefaultClient()
	if nil != err {
		return err
	}

	uploadURL := strings.TrimSuffix(repoURL, "/") + "/v1/packages"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("package", filepath.Base(archivePath))
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to create multipart form", err)
	}

	f, err := os.Open(archivePath)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrArchive, "failed to open archive for upload", err)
	}
	defer f.Close()

	if _, err := io.Copy(part, f); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to write archive to multipart form", err)
	}

	if err := writer.Close(); nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to close multipart writer", err)
	}

	// 5-minute upload timeout: large archives (multi-MB) over slow links
	// need room, but a hostile or stalled repo cannot hang keramos forever.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrRepo, "failed to create upload request", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.Do(req)
	if nil != err {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || 300 <= resp.StatusCode {
		// Bound the error-body read so a hostile repo can't OOM us by
		// returning gigabytes of error text.
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return keramoserr.NewErrorf(keramoserr.ErrRepo, "publish failed: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Published %s@%s to %s\n", meta.Name, meta.Version, repoURL)
	return nil
}
