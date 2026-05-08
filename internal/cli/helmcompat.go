package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/migrate"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newHelmCompatCommand exposes Helm-compat helpers: import a Helm chart into
// keramos syntax (delegates to migrate), and emit a compat report that explains
// which chart constructs translate cleanly.
func newHelmCompatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helm-compat",
		Short: "Helm chart compatibility helpers",
	}
	cmd.AddCommand(newHelmCompatReportCommand())
	cmd.AddCommand(newHelmCompatExportCommand())
	return cmd
}

func newHelmCompatReportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report <chart-path>",
		Short: "Analyse a Helm chart and report which constructs keramos supports natively",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := analyseChart(args[0])
			if nil != err {
				return err
			}
			data, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	return cmd
}

func newHelmCompatExportCommand() *cobra.Command {
	var dest string
	cmd := &cobra.Command{
		Use:   "export <keramos-package-path>",
		Short: "Export a keramos package as a Helm v3 chart for compat with helm tooling",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if "" == dest {
				dest = filepath.Join(os.TempDir(), "keramos-helm-export-"+filepath.Base(args[0]))
			}
			return exportToHelmChart(args[0], dest)
		},
	}
	cmd.Flags().StringVar(&dest, "out", "", "output directory for the Helm chart")
	return cmd
}

type helmCompatReport struct {
	Chart            string   `json:"chart"`
	Templates        int      `json:"templates"`
	GoTemplateBlocks int      `json:"goTemplateBlocks"`
	Notes            []string `json:"notes"`
	Recommendations  []string `json:"recommendations"`
}

func analyseChart(chartPath string) (*helmCompatReport, error) {
	rep := &helmCompatReport{Chart: filepath.Base(chartPath)}
	tmplDir := filepath.Join(chartPath, "templates")
	if _, err := os.Stat(tmplDir); nil != err {
		return nil, keramoserr.WrapError(keramoserr.ErrCLIValidation, "stat templates dir", err)
	}
	walkErr := filepath.Walk(tmplDir, func(path string, info os.FileInfo, e error) error {
		if nil != e || info.IsDir() {
			return e
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ".yaml" != ext && ".yml" != ext && ".tpl" != ext {
			return nil
		}
		rep.Templates++
		data, readErr := os.ReadFile(path)
		if nil != readErr {
			return keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, readErr, "read template %s", path)
		}
		body := string(data)
		blocks := strings.Count(body, "{{")
		rep.GoTemplateBlocks += blocks
		if 0 < blocks {
			rep.Notes = append(rep.Notes,
				fmt.Sprintf("%s: %d Go-template blocks (run 'keramos migrate' to translate)", filepath.Base(path), blocks))
		}
		return nil
	})
	if nil != walkErr {
		return nil, walkErr
	}
	if 0 < rep.GoTemplateBlocks {
		rep.Recommendations = append(rep.Recommendations,
			"Run 'keramos migrate "+chartPath+"' to translate go-template blocks to keramos's ${...} syntax")
	}
	rep.Recommendations = append(rep.Recommendations,
		"Sub-charts: drop them under <keramos-pkg>/charts/ unchanged; keramos layer-resolves them via dependencies in keramos.yaml")
	return rep, nil
}

// exportToHelmChart writes a Chart.yaml + values.yaml + templates/ tree that
// `helm` can render. keramos's `${expr}` blocks are emitted verbatim — they only
// resolve under keramos, so this is suitable for sharing static manifests, not
// for round-tripping templates back through helm. For full helm rendering the
// user should pre-render with `keramos template` and ship the static output.
func exportToHelmChart(pkg, dest string) error {
	if mkErr := os.MkdirAll(filepath.Join(dest, "templates"), 0o755); nil != mkErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "mkdir export", mkErr)
	}
	keramosData, err := os.ReadFile(filepath.Join(pkg, "keramos.yaml"))
	if nil != err {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "read keramos.yaml", err)
	}
	var meta map[string]any
	if uErr := yaml.Unmarshal(keramosData, &meta); nil != uErr {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse keramos.yaml", uErr)
	}
	chart := map[string]any{
		"apiVersion":  "v2",
		"name":        meta["name"],
		"version":     meta["version"],
		"description": meta["description"],
		"appVersion":  meta["appVersion"],
	}
	chartYaml, mErr := yaml.Marshal(chart)
	if nil != mErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "marshal Chart.yaml", mErr)
	}
	if wErr := os.WriteFile(filepath.Join(dest, "Chart.yaml"), chartYaml, 0o644); nil != wErr {
		return keramoserr.WrapError(keramoserr.ErrInternal, "write Chart.yaml", wErr)
	}
	if vData, err := os.ReadFile(filepath.Join(pkg, "values.yaml")); nil == err {
		if wErr := os.WriteFile(filepath.Join(dest, "values.yaml"), vData, 0o644); nil != wErr {
			return keramoserr.WrapError(keramoserr.ErrInternal, "write values.yaml", wErr)
		}
	}
	tmplDir := filepath.Join(pkg, "templates")
	walkErr := filepath.Walk(tmplDir, func(path string, info os.FileInfo, e error) error {
		if nil != e || info.IsDir() {
			return e
		}
		// Reject symlinks outright: filepath.Walk does not traverse them but
		// os.ReadFile/os.WriteFile below DO follow them, so a symlink named
		// foo.yaml → /etc/shadow would silently embed host secrets in the
		// exported chart.
		lstat, lerr := os.Lstat(path)
		if nil != lerr {
			return keramoserr.WrapError(keramoserr.ErrCLIValidation, "lstat template", lerr)
		}
		if 0 != lstat.Mode()&os.ModeSymlink {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"refusing to follow symlink in templates: %s", path)
		}
		rel, relErr := filepath.Rel(tmplDir, path)
		if nil != relErr {
			return keramoserr.WrapError(keramoserr.ErrCLIValidation, "relativise template path", relErr)
		}
		if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
				"template path %q escapes the templates directory", rel)
		}
		out := filepath.Join(dest, "templates", rel)
		if mkErr := os.MkdirAll(filepath.Dir(out), 0o755); nil != mkErr {
			return mkErr
		}
		body, err := os.ReadFile(path)
		if nil != err {
			return err
		}
		return os.WriteFile(out, body, 0o644)
	})
	if nil != walkErr {
		return walkErr
	}
	fmt.Printf("exported helm-compat chart to %s\n", dest)
	return nil
}

// _ keeps migrate import used (prevents go-mod tidy from removing it; the
// migrate command shares analysis helpers we may consume in future).
var _ = migrate.Migrate
