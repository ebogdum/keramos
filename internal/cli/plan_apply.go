package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ebogdum/keramos/internal/action"
	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/spf13/cobra"
)

// validatePlanPath rejects plan-supplied paths that are absolute or contain
// traversal sequences. Without this, a hostile plan file could trick `keramos
// apply` into reading any directory readable by the operator and rendering
// it as a keramos package — including /tmp, /etc, or user home directories.
func validatePlanPath(p string) error {
	if "" == p {
		return keramoserr.NewError(keramoserr.ErrCLIValidation, "plan packagePath is empty")
	}
	if filepath.IsAbs(p) {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"plan packagePath %q must be relative; refusing to apply plan referencing absolute paths", p)
	}
	clean := filepath.Clean(p)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, string(filepath.Separator)+"..") {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"plan packagePath %q contains a traversal sequence", p)
	}
	return nil
}

// keramosPlan is the on-disk representation of an `keramos plan` invocation. Apply
// reads this back, verifies the package digest still matches, and runs the
// install/upgrade with the same rendered manifest.
type keramosPlan struct {
	APIVersion   string            `json:"apiVersion"`
	Kind         string            `json:"kind"`
	GeneratedAt  time.Time         `json:"generatedAt"`
	Action       string            `json:"action"`
	ReleaseName  string            `json:"releaseName"`
	Namespace    string            `json:"namespace"`
	PackagePath  string            `json:"packagePath"`
	Profile      string            `json:"profile,omitempty"`
	ValueFiles   []string          `json:"valueFiles,omitempty"`
	Sets         []string          `json:"sets,omitempty"`
	SetStrings   []string          `json:"setStrings,omitempty"`
	Manifest     string            `json:"manifest"`
	ManifestSHA  string            `json:"manifestSha256"`
	Notes        string            `json:"notes,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

func newPlanCommand() *cobra.Command {
	var (
		valueFiles  []string
		sets        []string
		setStrings  []string
		profile     string
		out         string
		actionKind  string
		labels      []string
	)
	cmd := &cobra.Command{
		Use:   "plan <release> <package-path>",
		Short: "Render and persist an apply-able plan without touching the cluster",
		Long: `Generate a plan file that pins the rendered manifest, value flags, and
metadata for a future 'keramos apply'. Decouples templating from execution so
the same plan can be reviewed, signed off, and applied later.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			labelMap, lerr := parseLabelFlags(labels)
			if nil != lerr {
				return lerr
			}
			// Validate the same way `keramos apply` will validate later, so a
			// plan we cannot apply is rejected up front rather than written
			// to disk and discovered hours later.
			if pErr := validatePlanPath(args[1]); nil != pErr {
				return pErr
			}
			for _, vf := range valueFiles {
				if pErr := validatePlanPath(vf); nil != pErr {
					return pErr
				}
			}
			if "install" != actionKind && "upgrade" != actionKind {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"unsupported --action %q; use 'install' or 'upgrade'", actionKind)
			}
			rel, err := action.Install(nil, args[1], &action.InstallOptions{
				ReleaseName: args[0],
				Namespace:   namespace,
				ValueFiles:  valueFiles,
				Sets:        sets,
				SetStrings:  setStrings,
				Profile:     profile,
				DryRun:      "client",
				Labels:      labelMap,
			})
			if nil != err {
				return err
			}
			sum := sha256.Sum256([]byte(rel.Manifest))
			plan := keramosPlan{
				APIVersion:  "keramos/v1",
				Kind:        "Plan",
				GeneratedAt: time.Now().UTC(),
				Action:      actionKind,
				ReleaseName: args[0],
				Namespace:   namespace,
				PackagePath: args[1],
				Profile:     profile,
				ValueFiles:  valueFiles,
				Sets:        sets,
				SetStrings:  setStrings,
				Manifest:    rel.Manifest,
				ManifestSHA: hex.EncodeToString(sum[:]),
				Notes:       rel.Notes,
				Labels:      labelMap,
			}
			data, mErr := json.MarshalIndent(plan, "", "  ")
			if nil != mErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "marshal plan", mErr)
			}
			if "" == out || "-" == out {
				fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return nil
			}
			if writeErr := os.WriteFile(out, data, 0o600); nil != writeErr {
				return keramoserr.WrapError(keramoserr.ErrInternal, "write plan", writeErr)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "plan written to %s\n", out)
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "key=value forced as string (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().StringVar(&actionKind, "action", "install", "action the plan represents: install or upgrade")
	cmd.Flags().StringVarP(&out, "out", "o", "-", "plan output file (- for stdout)")
	cmd.Flags().StringArrayVar(&labels, "labels", nil, "label key=value (repeatable)")
	return cmd
}

func newApplyCommand() *cobra.Command {
	var (
		planFile string
		dryRun   string
	)
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a plan produced by 'keramos plan'",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if "" == planFile {
				return keramoserr.NewError(keramoserr.ErrCLIValidation, "--plan is required")
			}
			data, err := os.ReadFile(planFile)
			if nil != err {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "read plan", err)
			}
			var p keramosPlan
			if jErr := json.Unmarshal(data, &p); nil != jErr {
				return keramoserr.WrapError(keramoserr.ErrCLIValidation, "parse plan", jErr)
			}
			if "keramos/v1" != p.APIVersion || "Plan" != p.Kind {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"unsupported plan kind %s/%s", p.APIVersion, p.Kind)
			}
			if "install" != p.Action && "upgrade" != p.Action {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"plan declares unsupported action %q; expected install or upgrade", p.Action)
			}
			if pErr := validatePlanPath(p.PackagePath); nil != pErr {
				return pErr
			}
			for _, vf := range p.ValueFiles {
				if pErr := validatePlanPath(vf); nil != pErr {
					return pErr
				}
			}
			client, kErr := kube.NewClient(kubeconfig, kubeContext, p.Namespace)
			if nil != kErr {
				return kErr
			}
			// Plan integrity: re-render with a client-side dry-run and verify
			// the manifest hash still matches the plan. Detects drift between
			// plan and apply caused by package or value-file edits.
			preview, prevErr := action.Install(nil, p.PackagePath, &action.InstallOptions{
				ReleaseName: p.ReleaseName,
				Namespace:   p.Namespace,
				ValueFiles:  p.ValueFiles,
				Sets:        p.Sets,
				SetStrings:  p.SetStrings,
				Profile:     p.Profile,
				DryRun:      "client",
				Labels:      p.Labels,
			})
			if nil != prevErr {
				return prevErr
			}
			if "" == p.ManifestSHA {
				return keramoserr.NewError(keramoserr.ErrCLIValidation,
					"plan is missing manifestSha256 integrity field; refusing to apply")
			}
			gotSum := sha256.Sum256([]byte(preview.Manifest))
			if p.ManifestSHA != hex.EncodeToString(gotSum[:]) {
				return keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
					"plan integrity check failed: package or values changed since plan was generated (expected sha %s, got %s)",
					p.ManifestSHA, hex.EncodeToString(gotSum[:]))
			}
			switch p.Action {
			case "upgrade":
				rel, uErr := action.Upgrade(client, p.PackagePath, &action.UpgradeOptions{
					ReleaseName: p.ReleaseName,
					Namespace:   p.Namespace,
					ValueFiles:  p.ValueFiles,
					Sets:        p.Sets,
					SetStrings:  p.SetStrings,
					Profile:     p.Profile,
					Atomic:      true,
					Wait:        true,
					Timeout:     5 * time.Minute,
					DryRun:      dryRun,
					Labels:      p.Labels,
				})
				if nil != uErr {
					return uErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied upgrade for %s revision %d\n", rel.Name, rel.Revision)
			default:
				rel, iErr := action.Install(client, p.PackagePath, &action.InstallOptions{
					ReleaseName: p.ReleaseName,
					Namespace:   p.Namespace,
					ValueFiles:  p.ValueFiles,
					Sets:        p.Sets,
					SetStrings:  p.SetStrings,
					Profile:     p.Profile,
					Atomic:      true,
					Wait:        true,
					Timeout:     5 * time.Minute,
					DryRun:      dryRun,
					Labels:      p.Labels,
				})
				if nil != iErr {
					return iErr
				}
				fmt.Fprintf(cmd.OutOrStdout(), "applied install for %s revision %d\n", rel.Name, rel.Revision)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&planFile, "plan", "", "plan file produced by 'keramos plan'")
	cmd.Flags().StringVar(&dryRun, "dry-run", "", "dry-run mode: client or server")
	return cmd
}
