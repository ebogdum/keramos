package cli

import (
	"fmt"
	"path/filepath"

	"github.com/ebogdum/keramos/internal/action"
	"github.com/ebogdum/keramos/internal/engine"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/layer"
	"github.com/ebogdum/keramos/internal/logger"
	"github.com/ebogdum/keramos/internal/values"
	"github.com/spf13/cobra"
)

func newTemplateCommand() *cobra.Command {
	var (
		valueFiles  []string
		sets        []string
		setStrings  []string
		setFiles    []string
		setJSON     []string
		profile     string
		showOnly    []string
		releaseName string
		isUpgrade   bool
		validate    bool
		includeCRDs bool
		apiVersions []string
		kubeVersion string
		nameTpl     string
		postRender  string
		envName     string
	)

	cmd := &cobra.Command{
		Use:   "template <package-path>",
		Short: "Render keramos templates locally",
		Long:  "Render keramos templates locally without requiring a Kubernetes cluster.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTemplate(cmd, args[0], &templateOpts{
				ValueFiles:  valueFiles,
				Sets:        sets,
				SetStrings:  setStrings,
				SetFiles:    setFiles,
				SetJSON:     setJSON,
				Profile:     profile,
				ShowOnly:    showOnly,
				ReleaseName: releaseName,
				IsUpgrade:   isUpgrade,
				Validate:    validate,
				IncludeCRDs: includeCRDs,
				APIVersions: apiVersions,
				KubeVersion: kubeVersion,
				NameTpl:     nameTpl,
				PostRender:  postRender,
				Environment: envName,
			})
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "force string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; value read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value parsed as JSON (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().StringArrayVarP(&showOnly, "show-only", "s", nil, "render only the named template file (repeatable)")
	cmd.Flags().StringVar(&releaseName, "release-name", "", "override release name (default: package name)")
	cmd.Flags().BoolVar(&isUpgrade, "is-upgrade", false, "render with .Release.IsUpgrade = true")
	cmd.Flags().BoolVar(&validate, "validate", false, "validate against the cluster after rendering (server-side dry-run)")
	cmd.Flags().BoolVar(&includeCRDs, "include-crds", false, "include CRDs from crds/ in the output")
	cmd.Flags().StringArrayVar(&apiVersions, "api-versions", nil, "Kubernetes API version available for capability checks (repeatable)")
	cmd.Flags().StringVar(&kubeVersion, "kube-version", "", "override Kubernetes version reported in capabilities")
	cmd.Flags().StringVar(&nameTpl, "name-template", "", "name-template (currently equivalent to --release-name)")
	cmd.Flags().StringVar(&postRender, "post-renderer", "", "command piped the rendered manifests on stdin")
	cmd.Flags().StringVar(&envName, "env", "", "environment name declared in keramos.yaml's environments: section")

	return cmd
}

type templateOpts struct {
	ValueFiles  []string
	Sets        []string
	SetStrings  []string
	SetFiles    []string
	SetJSON     []string
	Profile     string
	ShowOnly    []string
	ReleaseName string
	IsUpgrade   bool
	Validate    bool
	IncludeCRDs bool
	APIVersions []string
	KubeVersion string
	NameTpl     string
	PostRender  string
	Environment string
}

func runTemplate(cmd *cobra.Command, packagePath string, o *templateOpts) error {
	logger.Debug("resolving package at %s", packagePath)

	if "" != o.Environment {
		overlay, envErr := action.ResolveEnvironmentOverlay(packagePath, o.Environment)
		if nil != envErr {
			return envErr
		}
		// Environment-declared profile/value-files take effect only when not
		// already overridden on the CLI; explicit --set-json from the env
		// folds in at lowest precedence.
		if "" == o.Profile && "" != overlay.Profile {
			o.Profile = overlay.Profile
		}
		if 0 < len(overlay.ValueFiles) {
			o.ValueFiles = append(overlay.ValueFiles, o.ValueFiles...)
		}
		if 0 < len(overlay.SetJSON) {
			o.SetJSON = append(overlay.SetJSON, o.SetJSON...)
		}
	}

	resolved, err := layer.Resolve(packagePath, o.Profile)
	if nil != err {
		return err
	}

	mergedValues, err := values.ResolveAll(map[string]any(resolved.Values),
		o.ValueFiles, o.Sets, o.SetStrings, o.SetFiles, o.SetJSON)
	if nil != err {
		return err
	}

	if schemaErr := action.ValidateValuesAgainstSchema(packagePath, mergedValues); nil != schemaErr {
		return schemaErr
	}

	releaseName := o.ReleaseName
	if "" == releaseName {
		releaseName = o.NameTpl
	}
	if "" == releaseName {
		releaseName = resolved.Metadata.Name
	}

	caps := map[string]any{}
	for _, v := range o.APIVersions {
		if mp, ok := caps["apiVersions"].(map[string]bool); ok {
			mp[v] = true
		} else {
			caps["apiVersions"] = map[string]bool{v: true}
		}
	}
	if "" != o.KubeVersion {
		caps["kubeVersion"] = map[string]any{
			"Version":    o.KubeVersion,
			"GitVersion": o.KubeVersion,
		}
	}

	ctx := &engine.RenderContext{
		Values: mergedValues,
		Package: map[string]any{
			"name":       resolved.Metadata.Name,
			"version":    resolved.Metadata.Version,
			"appVersion": resolved.Metadata.AppVersion,
		},
		Release: map[string]any{
			"name":      releaseName,
			"namespace": namespace,
			"revision":  1,
			"isUpgrade": o.IsUpgrade,
			"isInstall": !o.IsUpgrade,
		},
		Capabilities: caps,
		Files:        resolved.Files,
	}

	templates := resolved.Templates
	if 0 < len(o.ShowOnly) {
		filtered := make(map[string]string)
		for _, want := range o.ShowOnly {
			matched := false
			for name, content := range resolved.Templates {
				if name == want || filepath.Base(name) == want {
					filtered[name] = content
					matched = true
				}
			}
			if !matched {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "template file not found: %s", want)
			}
		}
		templates = filtered
	}

	eng := engine.New()
	output, err := eng.Render(templates, resolved.Partials, ctx)
	if nil != err {
		return err
	}

	if o.IncludeCRDs {
		crdSection, crdErr := action.LoadCRDsForRender(packagePath)
		if nil != crdErr {
			return crdErr
		}
		if "" != crdSection {
			output = crdSection + "---\n" + output
		}
	}

	if "" != o.PostRender {
		out, prErr := action.RunPostRendererPublic(o.PostRender, output)
		if nil != prErr {
			return prErr
		}
		output = out
	}

	if o.Validate {
		client, kubeErr := newKubeClientForTemplate()
		if nil != kubeErr {
			return kubeErr
		}
		if dryRunErr := client.DryRunApply(output); nil != dryRunErr {
			return keramoserr.WrapError(keramoserr.ErrCLIValidation, "validation against cluster failed", dryRunErr)
		}
	}

	fmt.Fprint(cmd.OutOrStdout(), output)
	return nil
}

// newKubeClientForTemplate constructs a real kube.Client for `--validate` runs.
func newKubeClientForTemplate() (kubeClientForTemplate, error) {
	return kube.NewClient(kubeconfig, kubeContext, namespace)
}

type kubeClientForTemplate interface {
	DryRunApply(string) error
}
