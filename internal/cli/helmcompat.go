package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/helmcompat"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/ebogdum/keramos/v3/internal/values"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// newHelmCompatCommand exposes Helm-compat helpers: render/install an
// unmodified upstream Helm chart under keramos's release record, import a Helm
// chart into keramos syntax (delegates to migrate), and emit a compat report.
func newHelmCompatCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "helm-compat",
		Short: "Run and convert upstream Helm charts",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newHelmCompatRenderCommand())
	cmd.AddCommand(newHelmCompatInstallCommand())
	cmd.AddCommand(newHelmCompatReportCommand())
	cmd.AddCommand(newHelmCompatExportCommand())
	return cmd
}

// renderHelmChart loads values flags, queries cluster capabilities when a
// client is available, and renders the chart via the helmcompat engine,
// returning the manifests joined in stable filename order.
func renderHelmChart(chartPath string, valueFiles, sets, setStrings, setFiles, setJSON []string, rel helmcompat.ReleaseMeta, caps helmcompat.CapabilitiesMeta) (string, error) {
	userValues, err := values.ResolveAll(map[string]any{}, valueFiles, sets, setStrings, setFiles, setJSON)
	if nil != err {
		return "", err
	}
	rendered, err := helmcompat.Render(chartPath, helmcompat.Options{
		Release:      rel,
		Capabilities: caps,
		UserValues:   userValues,
	})
	if nil != err {
		return "", err
	}
	names := make([]string, 0, len(rendered))
	for n := range rendered {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "---\n# Source: %s\n%s\n", n, strings.TrimRight(rendered[n], "\n"))
	}
	return b.String(), nil
}

func newHelmCompatRenderCommand() *cobra.Command {
	var (
		name        string
		namespace   string
		kubeconfig  string
		kubeContext string
		valueFiles  []string
		sets        []string
		setStrings  []string
		setFiles    []string
		setJSON     []string
		kubeVersion string
	)
	cmd := &cobra.Command{
		Use:   "render <chart-path>",
		Short: "Render an unmodified Helm chart to manifests (like `helm template`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			caps := helmcompat.CapabilitiesMeta{KubeVersion: kubeVersion}
			// Best-effort cluster capabilities (offline render still works).
			if client, cErr := kube.NewClient(kubeconfig, kubeContext, namespace); nil == cErr {
				if c, capErr := client.GetCapabilities(); nil == capErr {
					caps = capabilitiesFromMap(c, kubeVersion)
				}
			}
			rel := helmcompat.ReleaseMeta{Name: name, Namespace: namespace, Revision: 1, IsInstall: true}
			out, err := renderHelmChart(args[0], valueFiles, sets, setStrings, setFiles, setJSON, rel, caps)
			if nil != err {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "release-name", "release name (.Release.Name)")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "namespace (.Release.Namespace)")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kubeconfig context")
	cmd.Flags().StringSliceVarP(&valueFiles, "values", "f", nil, "values file(s)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set values (key=val)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set string values")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set values from files")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set JSON values")
	cmd.Flags().StringVar(&kubeVersion, "kube-version", "", "override .Capabilities.KubeVersion for offline render")
	return cmd
}

func newHelmCompatInstallCommand() *cobra.Command {
	var (
		namespace   string
		kubeconfig  string
		kubeContext string
		valueFiles  []string
		sets        []string
		setStrings  []string
		setFiles    []string
		setJSON     []string
	)
	cmd := &cobra.Command{
		Use:   "install <name> <chart-path>",
		Short: "Render an unmodified Helm chart and apply it under a keramos release record",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, chartPath := args[0], args[1]
			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			storage, storageErr := release.SelectStorage(client.Clientset(), namespace)
			if nil != storageErr {
				return storageErr
			}
			if existing, _ := storage.Last(name); nil != existing {
				return keramoserr.NewErrorf(keramoserr.ErrRelease, "release %q already exists; use a different name", name)
			}

			caps := helmcompat.CapabilitiesMeta{}
			if c, capErr := client.GetCapabilities(); nil == capErr {
				caps = capabilitiesFromMap(c, "")
			}
			rel := helmcompat.ReleaseMeta{Name: name, Namespace: namespace, Revision: 1, IsInstall: true}
			manifest, err := renderHelmChart(chartPath, valueFiles, sets, setStrings, setFiles, setJSON, rel, caps)
			if nil != err {
				return err
			}
			if applyErr := client.ApplyManifests(manifest); nil != applyErr {
				return applyErr
			}
			now := time.Now().UTC()
			rec := &release.Release{
				Name:      name,
				Namespace: namespace,
				Revision:  1,
				Status:    release.StatusDeployed,
				Manifest:  manifest,
				Info:      release.ReleaseInfo{FirstDeployed: now, LastDeployed: now, Description: "helm-compat install"},
			}
			if storeErr := storage.Create(rec); nil != storeErr {
				return storeErr
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed Helm chart %s as keramos release %q (revision 1)\n", filepath.Base(chartPath), name)
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "namespace")
	cmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "path to kubeconfig")
	cmd.Flags().StringVar(&kubeContext, "kube-context", "", "kubeconfig context")
	cmd.Flags().StringSliceVarP(&valueFiles, "values", "f", nil, "values file(s)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set values (key=val)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set string values")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set values from files")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set JSON values")
	return cmd
}

// capabilitiesFromMap adapts the kube client's capability map to the
// helmcompat CapabilitiesMeta. kubeVersionOverride wins when non-empty.
func capabilitiesFromMap(c map[string]any, kubeVersionOverride string) helmcompat.CapabilitiesMeta {
	out := helmcompat.CapabilitiesMeta{KubeVersion: kubeVersionOverride}
	if "" == out.KubeVersion {
		if kv, ok := c["kubeVersion"].(string); ok {
			out.KubeVersion = kv
		}
	}
	switch av := c["apiVersions"].(type) {
	case []string:
		out.APIVersions = av
	case []any:
		for _, x := range av {
			if s, ok := x.(string); ok {
				out.APIVersions = append(out.APIVersions, s)
			}
		}
	}
	return out
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
