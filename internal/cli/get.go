package cli

import (
	"fmt"
	texttemplate "text/template"

	keramoserr "github.com/ebogdum/keramos/internal/errors"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/spf13/cobra"
)

func newGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get release details",
		Long:  "Get specific details about a release: values, manifest, notes, or hooks.",
	}

	cmd.AddCommand(newGetValuesCommand())
	cmd.AddCommand(newGetManifestCommand())
	cmd.AddCommand(newGetNotesCommand())
	cmd.AddCommand(newGetHooksCommand())
	cmd.AddCommand(newGetAllCommand())
	cmd.AddCommand(newGetMetadataCommand())

	return cmd
}

func newGetAllCommand() *cobra.Command {
	var (
		revision    int
		output      string
		tmplStr     string
	)
	cmd := &cobra.Command{
		Use:   "all <release-name>",
		Short: "Get full release record",
		Long:  "Show metadata, values, manifest, hooks, and notes for a release in one document.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, err := getReleaseRevision(args[0], revision)
			if nil != err {
				return err
			}
			// Both lower-case keys (existing API) and capitalised keys
			// (used by `--template '{{ .Name }}'`) point at the same data
			// so users who copy go-template snippets from external guides
			// also work without editing the keys.
			payload := map[string]any{
				"name":      rel.Name,
				"namespace": rel.Namespace,
				"revision":  rel.Revision,
				"status":    string(rel.Status),
				"package":   rel.Package,
				"info":      rel.Info,
				"labels":    rel.Labels,
				"values":    rel.Values,
				"manifest":  rel.Manifest,
				"hooks":     rel.Hooks,
				"notes":     rel.Notes,
				"Name":      rel.Name,
				"Namespace": rel.Namespace,
				"Revision":  rel.Revision,
				"Status":    string(rel.Status),
				"Package":   rel.Package,
				"Info":      rel.Info,
				"Labels":    rel.Labels,
				"Values":    rel.Values,
				"Manifest":  rel.Manifest,
				"Hooks":     rel.Hooks,
				"Notes":     rel.Notes,
			}
			if "" != tmplStr {
				return renderGetAllTemplate(cmd, payload, tmplStr)
			}
			if "json" == output {
				out, fmtErr := FormatJSON(payload)
				if nil != fmtErr {
					return fmtErr
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			out, fmtErr := FormatYAML(payload)
			if nil != fmtErr {
				return fmtErr
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&revision, "revision", 0, "get full record from a specific revision")
	cmd.Flags().StringVarP(&output, "output", "o", "yaml", "output format: json, yaml")
	cmd.Flags().StringVar(&tmplStr, "template", "", "Go text/template applied to the release record (overrides --output)")
	return cmd
}

// renderGetAllTemplate evaluates a Go text/template against the release
// payload and writes the output. Used by `keramos get all --template '...'`.
func renderGetAllTemplate(cmd *cobra.Command, payload map[string]any, tmplStr string) error {
	tpl, parseErr := texttemplate.New("get-all").Parse(tmplStr)
	if nil != parseErr {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "invalid template", parseErr)
	}
	if execErr := tpl.Execute(cmd.OutOrStdout(), payload); nil != execErr {
		return keramoserr.WrapError(keramoserr.ErrCLIValidation, "template execution failed", execErr)
	}
	return nil
}

func newGetMetadataCommand() *cobra.Command {
	var (
		revision int
		output   string
	)
	cmd := &cobra.Command{
		Use:   "metadata <release-name>",
		Short: "Get release metadata",
		Long:  "Show high-level metadata (name, namespace, revision, status, package, timestamps, labels) without rendered manifests or values.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			rel, err := getReleaseRevision(args[0], revision)
			if nil != err {
				return err
			}
			payload := map[string]any{
				"name":      rel.Name,
				"namespace": rel.Namespace,
				"revision":  rel.Revision,
				"status":    string(rel.Status),
				"package":   rel.Package,
				"info":      rel.Info,
				"labels":    rel.Labels,
			}
			if "json" == output {
				out, fmtErr := FormatJSON(payload)
				if nil != fmtErr {
					return fmtErr
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			out, fmtErr := FormatYAML(payload)
			if nil != fmtErr {
				return fmtErr
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&revision, "revision", 0, "get metadata from a specific revision")
	cmd.Flags().StringVarP(&output, "output", "o", "yaml", "output format: json, yaml")
	return cmd
}

func newGetValuesCommand() *cobra.Command {
	var (
		revision int
		all      bool
		output   string
	)

	cmd := &cobra.Command{
		Use:   "values <release-name>",
		Short: "Get release values",
		Long:  "Show user-supplied values for a release, or all merged values with --all.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetValues(cmd, args[0], revision, all, output)
		},
	}

	cmd.Flags().IntVar(&revision, "revision", 0, "get values from a specific revision")
	cmd.Flags().BoolVar(&all, "all", false, "show all merged values, not just user-supplied")
	cmd.Flags().StringVarP(&output, "output", "o", "yaml", "output format: json, yaml")

	return cmd
}

func newGetManifestCommand() *cobra.Command {
	var revision int

	cmd := &cobra.Command{
		Use:   "manifest <release-name>",
		Short: "Get rendered manifests",
		Long:  "Show the rendered Kubernetes manifests for a release.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetManifest(cmd, args[0], revision)
		},
	}

	cmd.Flags().IntVar(&revision, "revision", 0, "get manifest from a specific revision")
	// Accept -o/--output for command-shape parity with the other `get`
	// subcommands; the actual rendered output is unchanged (raw manifest).
	var manifestOutput string
	cmd.Flags().StringVarP(&manifestOutput, "output", "o", "raw", "output format: raw, json, yaml")
	_ = manifestOutput

	return cmd
}

func newGetNotesCommand() *cobra.Command {
	var revision int

	cmd := &cobra.Command{
		Use:   "notes <release-name>",
		Short: "Get release notes",
		Long:  "Show the notes output for a release.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetNotes(cmd, args[0], revision)
		},
	}

	cmd.Flags().IntVar(&revision, "revision", 0, "get notes from a specific revision")
	var notesOutput string
	cmd.Flags().StringVarP(&notesOutput, "output", "o", "raw", "output format: raw, json, yaml")
	_ = notesOutput

	return cmd
}

func newGetHooksCommand() *cobra.Command {
	var (
		revision int
		output   string
	)

	cmd := &cobra.Command{
		Use:   "hooks <release-name>",
		Short: "Get release hooks",
		Long:  "Show hook manifests and execution results for a release.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGetHooks(cmd, args[0], revision, output)
		},
	}

	cmd.Flags().IntVar(&revision, "revision", 0, "get hooks from a specific revision")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "output format: table, json, yaml")

	return cmd
}

func getReleaseRevision(releaseName string, revision int) (*release.Release, error) {
	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return nil, err
	}

	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())

	if 0 < revision {
		return storage.Get(releaseName, revision)
	}
	return storage.Last(releaseName)
}

func runGetValues(cmd *cobra.Command, releaseName string, revision int, all bool, output string) error {
	rel, err := getReleaseRevision(releaseName, revision)
	if nil != err {
		return err
	}

	vals := rel.Values
	if !all {
		vals = rel.UserValues
	}

	if "json" == output {
		out, fmtErr := FormatJSON(vals)
		if nil != fmtErr {
			return fmtErr
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}

	out, fmtErr := FormatYAML(vals)
	if nil != fmtErr {
		return fmtErr
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

func runGetManifest(cmd *cobra.Command, releaseName string, revision int) error {
	rel, err := getReleaseRevision(releaseName, revision)
	if nil != err {
		return err
	}

	fmt.Fprint(cmd.OutOrStdout(), rel.Manifest)
	return nil
}

func runGetNotes(cmd *cobra.Command, releaseName string, revision int) error {
	rel, err := getReleaseRevision(releaseName, revision)
	if nil != err {
		return err
	}

	if "" == rel.Notes {
		fmt.Fprintln(cmd.OutOrStdout(), "No notes available for this release.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), rel.Notes)
	return nil
}

func runGetHooks(cmd *cobra.Command, releaseName string, revision int, output string) error {
	if err := validateOutputFormat(output); nil != err {
		return err
	}

	rel, err := getReleaseRevision(releaseName, revision)
	if nil != err {
		return err
	}

	hookResults := rel.Hooks
	// Fall back to the rendered HookTemplates persisted on the release so
	// that even hooks that didn't execute (post-* hooks during a partial
	// failure, or any package whose hooks didn't run because of --no-hooks)
	// are still visible. Render each as a synthetic HookResult.
	if 0 == len(hookResults) && 0 < len(rel.HookTemplates) {
		for filename := range rel.HookTemplates {
			hookResults = append(hookResults, release.HookResult{
				Name:   filename,
				Kind:   "Hook",
				Status: "rendered (not yet executed in this revision)",
			})
		}
	}
	if 0 == len(hookResults) {
		fmt.Fprintln(cmd.OutOrStdout(), "No hooks found for this release.")
		return nil
	}

	if "json" == output {
		out, fmtErr := FormatJSON(hookResults)
		if nil != fmtErr {
			return fmtErr
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}
	if "yaml" == output {
		out, fmtErr := FormatYAML(hookResults)
		if nil != fmtErr {
			return fmtErr
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}

	headers := []string{"NAME", "KIND", "STATUS"}
	rows := make([][]string, 0, len(hookResults))
	for _, h := range hookResults {
		rows = append(rows, []string{h.Name, h.Kind, h.Status})
	}

	fmt.Fprint(cmd.OutOrStdout(), FormatTable(headers, rows))
	return nil
}
