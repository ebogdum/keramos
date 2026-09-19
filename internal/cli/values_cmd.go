package cli

import (
	"fmt"

	"github.com/ebogdum/keramos/v2/internal/layer"
	"github.com/ebogdum/keramos/v2/internal/values"
	"github.com/spf13/cobra"
)

// newValuesCommand exposes the values-resolution graph for a package +
// override set, optionally tracing how a single key's value was determined.
//
// Example:
//
//	keramos values mypkg --set replicas=5 -f overrides.yaml --trace replicas
//
// Output lists every contribution in resolution order (defaults → layers →
// values files → --set), highlighting the winning value with `→`.
func newValuesCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		setStrings []string
		setFiles   []string
		setJSON    []string
		profile    string
		trace      string
		output     string
	)
	cmd := &cobra.Command{
		Use:   "values <package-path>",
		Short: "Show the merged values and (optionally) trace per-key resolution",
		Long: `Resolve the values for a package as 'install' would (defaults → layers → -f
files → --set / --set-string / --set-file / --set-json overrides) and print
the merged result.

With --trace <dotted.key> keramos prints the resolution chain for that key only:
every contributor in the order it was applied, with the winning value marked.
This answers the universal operator question "where did Values.replicas=3 come
from?".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, err := layer.Resolve(args[0], profile)
			if nil != err {
				return err
			}
			merged, traceMap, err := values.ResolveAllWithTrace(
				map[string]any(resolved.Values), valueFiles, sets, setStrings, setFiles, setJSON,
			)
			if nil != err {
				return err
			}
			if "" != trace {
				fmt.Fprint(cmd.OutOrStdout(), values.FormatTrace(traceMap, trace))
				return nil
			}
			if "json" == output {
				out, fmtErr := FormatJSON(merged)
				if nil != fmtErr {
					return fmtErr
				}
				fmt.Fprint(cmd.OutOrStdout(), out)
				return nil
			}
			out, fmtErr := FormatYAML(merged)
			if nil != fmtErr {
				return fmtErr
			}
			fmt.Fprint(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set key=value forcing string interpretation")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; value is read from path")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value parsed as JSON")
	cmd.Flags().StringVar(&profile, "profile", "", "profile to apply")
	cmd.Flags().StringVar(&trace, "trace", "", "dotted key path; show only its resolution chain")
	cmd.Flags().StringVarP(&output, "output", "o", "yaml", "output format: yaml, json (ignored when --trace is set)")
	return cmd
}
