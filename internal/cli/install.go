package cli

import (
	"crypto/rand"
	"fmt"
	"time"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"

	"github.com/ebogdum/keramos/v3/internal/action"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/logger"
	"github.com/ebogdum/keramos/v3/internal/pkg"
	"github.com/ebogdum/keramos/v3/internal/repo"
	"github.com/spf13/cobra"
)

func newInstallCommand() *cobra.Command {
	var (
		valueFiles          []string
		sets                []string
		setStrings          []string
		setFiles            []string
		setJSON             []string
		profile             string
		noWait              bool
		explicitWait        bool
		timeout             time.Duration
		dryRun              string
		description         string
		noAtomic            bool
		noForce             bool
		noHooks             bool
		createNamespace     bool
		includeCRDs         bool
		skipCRDs            bool
		labels              []string
		apiVersions         []string
		kubeVersion         string
		postRenderer        string
		postRenderers       []string
		postRendererTimeout time.Duration
		cleanupOnFail       bool
		waitForJobs         bool
		hookTimeout         time.Duration
		keyring             string
		envName             string
		output              string
		generateName        bool
		verify              bool
		skipRequires        bool
		historyMax          int
		recreatePods        bool
		force               bool
	)

	cmd := &cobra.Command{
		Use:   "install <release-name> <package-path>",
		Short: "Install a keramos package as a new release",
		Long:  "Install a keramos package to the Kubernetes cluster as a new named release.",
		Args: func(cmd *cobra.Command, args []string) error {
			if generateName {
				if 1 != len(args) {
					return fmt.Errorf("when --generate-name is set, exactly 1 argument (package path) is required")
				}
				return nil
			}
			if 2 != len(args) {
				return fmt.Errorf("exactly 2 arguments required: <release-name> <package-path>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var releaseName, packagePath string

			if generateName {
				packagePath = args[0]
				baseName, nameErr := resolveBaseName(packagePath)
				if nil != nameErr {
					return nameErr
				}
				suffix, suffErr := randomSuffix(5)
				if nil != suffErr {
					return suffErr
				}
				releaseName = baseName + "-" + suffix
			} else {
				releaseName = args[0]
				packagePath = args[1]
			}

			if err := validateDryRunFlag(dryRun); nil != err {
				return err
			}
			if err := validateOutputFlag(output); nil != err {
				return err
			}

			if depErr := repo.ResolveDependencies(packagePath); nil != depErr {
				return depErr
			}

			if verify {
				if err := verifyInstalledSignaturesWithKeyring(packagePath, keyring); nil != err {
					return err
				}
			}

			labelMap, labelErr := parseLabelFlags(labels)
			if nil != labelErr {
				return labelErr
			}

			opts := &action.InstallOptions{
				ReleaseName:         releaseName,
				Namespace:           namespace,
				ValueFiles:          valueFiles,
				Sets:                sets,
				SetStrings:          setStrings,
				SetFiles:            setFiles,
				SetJSON:             setJSON,
				Profile:             profile,
				Wait:                explicitWait || !noWait,
				Timeout:             timeout,
				DryRun:              dryRun,
				Description:         description,
				Atomic:              !noAtomic,
				NoHooks:             noHooks,
				CreateNamespace:     createNamespace,
				IncludeCRDs:         includeCRDs && !skipCRDs,
				Labels:              labelMap,
				APIVersions:         apiVersions,
				KubeVersion:         kubeVersion,
				PostRenderer:        postRenderer,
				PostRenderers:       postRenderers,
				PostRendererTimeout: postRendererTimeout,
				CleanupOnFail:       cleanupOnFail,
				WaitForJobs:         waitForJobs,
				HookTimeout:         hookTimeout,
				Environment:         envName,
				SkipRequires:        skipRequires,
				HistoryMax:          historyMax,
				RecreatePods:        recreatePods,
				Force:               force,
			}

			if "client" == dryRun {
				rel, err := action.Install(nil, packagePath, opts)
				if nil != err {
					return err
				}
				return outputRelease(cmd.OutOrStdout(), rel, output, func() {
					fmt.Fprint(cmd.OutOrStdout(), rel.Manifest)
					if "" != rel.Notes {
						fmt.Fprintf(cmd.OutOrStdout(), "\nNOTES:\n%s\n", rel.Notes)
					}
				})
			}

			client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
			if nil != err {
				return err
			}
			if noForce {
				client.SetForce(false)
			}

			rel, err := action.Install(client, packagePath, opts)
			if nil != err {
				return err
			}

			return outputRelease(cmd.OutOrStdout(), rel, output, func() {
				logger.Log("release %s installed (revision %d)", rel.Name, rel.Revision)
				if "" != rel.Notes {
					fmt.Fprintf(cmd.OutOrStdout(), "\nNOTES:\n%s\n", rel.Notes)
				}
			})
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set key=value overrides forcing string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; the value is read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value is parsed as a JSON literal (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "don't wait for resources to be ready")
	cmd.Flags().BoolVar(&explicitWait, "wait", false, "wait for resources to be ready (default behaviour)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "timeout for readiness wait")
	cmd.Flags().StringVar(&dryRun, "dry-run", "", "dry-run mode: 'client' (local render) or 'server' (API validation)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")
	cmd.Flags().StringVar(&description, "description", "", "release description")
	cmd.Flags().BoolVar(&noAtomic, "no-atomic", false, "don't roll back on failure")
	cmd.Flags().BoolVar(&noForce, "no-force", false, "don't force field ownership on server-side apply")
	cmd.Flags().BoolVar(&noHooks, "no-hooks", false, "skip lifecycle hooks for this operation")
	cmd.Flags().BoolVar(&createNamespace, "create-namespace", false, "create the release namespace if missing")
	cmd.Flags().BoolVar(&skipCRDs, "skip-crds", false, "do not install the CustomResourceDefinitions in crds/")
	cmd.Flags().BoolVar(&includeCRDs, "include-crds", true, "install the CustomResourceDefinitions in crds/ before the templates")
	_ = cmd.Flags().MarkDeprecated("include-crds", "crds/ is installed by default; use --skip-crds to opt out")
	cmd.Flags().StringArrayVar(&labels, "labels", nil, "label key=value to attach to the release (repeatable)")
	cmd.Flags().StringArrayVar(&apiVersions, "api-versions", nil, "Kubernetes API version available for capability checks (repeatable)")
	cmd.Flags().StringVar(&kubeVersion, "kube-version", "", "override Kubernetes version reported in capabilities")
	cmd.Flags().StringVar(&postRenderer, "post-renderer", "", "command piped the rendered manifests on stdin (yields stdout)")
	cmd.Flags().StringArrayVar(&postRenderers, "post-renderers", nil, "chained post-renderers (repeatable; output of N feeds N+1)")
	cmd.Flags().DurationVar(&postRendererTimeout, "post-renderer-timeout", 5*time.Minute, "per-stage timeout for post-renderers")
	cmd.Flags().BoolVar(&cleanupOnFail, "cleanup-on-fail", false, "delete partially-applied resources if the install fails")
	cmd.Flags().BoolVar(&waitForJobs, "wait-for-jobs", false, "wait for Job resources to complete (in addition to --wait)")
	cmd.Flags().DurationVar(&hookTimeout, "hook-timeout", 0, "cap each hook's per-hook timeout (0 = use the chart-declared value)")
	cmd.Flags().StringVar(&keyring, "keyring", "", "path to PGP keyring directory for --verify (default: ~/.config/keramos/keyring)")
	cmd.Flags().StringVar(&envName, "env", "", "environment name declared in keramos.yaml's environments: section (replaces values-{env}.yaml)")
	cmd.Flags().BoolVar(&generateName, "generate-name", false, "generate a release name from the package name")
	cmd.Flags().BoolVar(&verify, "verify", false, "verify package signatures before installing")
	cmd.Flags().BoolVar(&skipRequires, "skip-requires", false, "skip installation of required co-deployed packages")
	cmd.Flags().IntVar(&historyMax, "history-max", 0, "maximum number of revisions to retain in history (0 = unlimited)")
	cmd.Flags().BoolVar(&recreatePods, "recreate-pods", false, "trigger a rolling restart of Deployments/StatefulSets/DaemonSets")
	cmd.Flags().BoolVar(&force, "force", false, "delete and recreate resources to force update of immutable fields")

	return cmd
}

// resolveBaseName loads the package metadata to get the package name for --generate-name.
func resolveBaseName(packagePath string) (string, error) {
	meta, err := pkg.LoadPackageMetadata(packagePath)
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrPackageInvalid, "failed to load package metadata for name generation", err)
	}
	if "" == meta.Name {
		return "", keramoserr.NewError(keramoserr.ErrPackageInvalid, "package has no name; cannot generate release name")
	}
	return meta.Name, nil
}

// randomSuffix generates a cryptographically random lowercase alphanumeric string of length n.
func randomSuffix(n int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	charsetLen := len(charset)
	buf := make([]byte, n)
	randomBytes := make([]byte, n)
	if _, err := rand.Read(randomBytes); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "failed to generate random suffix", err)
	}
	for i := 0; i < n; i++ {
		buf[i] = charset[int(randomBytes[i])%charsetLen]
	}
	return string(buf), nil
}
