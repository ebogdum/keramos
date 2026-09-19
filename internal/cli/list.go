package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	keramoserr "github.com/ebogdum/keramos/v3/internal/errors"
	"github.com/ebogdum/keramos/v3/internal/kube"
	"github.com/ebogdum/keramos/v3/internal/release"
	"github.com/spf13/cobra"
)

type listFilter struct {
	allNamespaces bool
	allStatuses   bool
	deployed      bool
	failed        bool
	pending       bool
	uninstalled   bool
	uninstalling  bool
	superseded    bool
	short         bool
	max           int
	offset        int
	filter        string
	selector      string
	byDate        bool
	output        string
	sortBy        string
	reverse       bool
}

func newListCommand() *cobra.Command {
	var f listFilter

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List releases",
		Long:    "List releases deployed to the cluster.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, &f)
		},
	}

	cmd.Flags().BoolVarP(&f.allNamespaces, "all-namespaces", "A", false, "list across all namespaces")
	cmd.Flags().BoolVarP(&f.allStatuses, "all", "a", false, "show all statuses including superseded and failed")
	cmd.Flags().BoolVar(&f.deployed, "deployed", false, "show only deployed releases")
	cmd.Flags().BoolVar(&f.failed, "failed", false, "show only failed releases")
	cmd.Flags().BoolVar(&f.pending, "pending", false, "show only pending releases")
	cmd.Flags().BoolVar(&f.uninstalled, "uninstalled", false, "show only uninstalled releases (with --keep-history)")
	cmd.Flags().BoolVar(&f.uninstalling, "uninstalling", false, "show only releases currently being uninstalled")
	cmd.Flags().BoolVar(&f.superseded, "superseded", false, "show only superseded releases")
	cmd.Flags().BoolVarP(&f.short, "short", "q", false, "output release names only")
	cmd.Flags().IntVarP(&f.max, "max", "m", 0, "maximum number of releases to display (0 = unlimited)")
	cmd.Flags().IntVar(&f.offset, "offset", 0, "skip the first N releases after sorting")
	cmd.Flags().StringVar(&f.filter, "filter", "", "regex filter on release name")
	cmd.Flags().StringVarP(&f.selector, "selector", "l", "", "label selector (key=value,...) applied to release labels")
	cmd.Flags().BoolVarP(&f.byDate, "date", "d", false, "shortcut for --sort-by date")
	cmd.Flags().StringVarP(&f.output, "output", "o", "table", "output format: table, json, yaml")
	cmd.Flags().StringVar(&f.sortBy, "sort-by", "name", "sort by: name, date, revision")
	cmd.Flags().BoolVar(&f.reverse, "reverse", false, "reverse the sort order")

	return cmd
}

func runList(cmd *cobra.Command, f *listFilter) error {
	if err := validateOutputFormat(f.output); nil != err {
		return err
	}

	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return err
	}

	ns := namespace
	if "" == ns {
		ns = client.Namespace()
	}
	if f.allNamespaces {
		ns = ""
	}

	storage, storageErr := release.SelectStorage(client.Clientset(), client.Namespace())
	if nil != storageErr {
		return storageErr
	}
	releases, err := storage.List(ns)
	if nil != err {
		return err
	}

	// Keep only the latest revision per release name.
	latest := latestRevisions(releases)

	// Status filtering: explicit per-status flags take precedence over --all.
	statusFilter := buildStatusFilter(f)
	if 0 < len(statusFilter) {
		latest = filterByStatus(latest, statusFilter)
	} else if !f.allStatuses {
		latest = filterActiveStatuses(latest)
	}

	if "" != f.filter {
		latest, err = filterByRegex(latest, f.filter)
		if nil != err {
			return err
		}
	}

	if "" != f.selector {
		labelSelectors, parseErr := parseLabelSelector(f.selector)
		if nil != parseErr {
			return parseErr
		}
		latest = filterByLabels(latest, labelSelectors)
	}

	sortKey := f.sortBy
	if f.byDate {
		sortKey = "date"
	}
	sortReleases(latest, sortKey)
	if f.reverse {
		for i, j := 0, len(latest)-1; i < j; i, j = i+1, j-1 {
			latest[i], latest[j] = latest[j], latest[i]
		}
	}

	if 0 < f.offset {
		if f.offset >= len(latest) {
			latest = nil
		} else {
			latest = latest[f.offset:]
		}
	}
	if 0 < f.max && f.max < len(latest) {
		latest = latest[:f.max]
	}

	if f.short {
		for _, rel := range latest {
			fmt.Fprintln(cmd.OutOrStdout(), rel.Name)
		}
		return nil
	}

	return outputReleases(cmd, latest, f.output)
}

func buildStatusFilter(f *listFilter) map[release.Status]bool {
	out := make(map[release.Status]bool)
	if f.deployed {
		out[release.StatusDeployed] = true
	}
	if f.failed {
		out[release.StatusFailed] = true
	}
	if f.pending {
		out[release.StatusPendingInstall] = true
		out[release.StatusPendingUpgrade] = true
		out[release.StatusPendingRollback] = true
	}
	if f.uninstalled {
		// Keramos marks uninstalled-but-kept history as Superseded; treat
		// the --uninstalled and --superseded filter flags as synonyms.
		out[release.StatusSuperseded] = true
	}
	if f.uninstalling {
		out[release.StatusUninstalling] = true
	}
	if f.superseded {
		out[release.StatusSuperseded] = true
	}
	return out
}

func filterByStatus(releases []*release.Release, allowed map[release.Status]bool) []*release.Release {
	out := make([]*release.Release, 0, len(releases))
	for _, rel := range releases {
		if allowed[rel.Status] {
			out = append(out, rel)
		}
	}
	return out
}

// parseLabelSelector parses a comma-separated `key=value` selector string.
// Equality only; set-matching (`key in (a,b)`) is intentionally not supported
// since release-record labels rarely benefit from it.
func parseLabelSelector(raw string) (map[string]string, error) {
	out := make(map[string]string)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if "" == part {
			continue
		}
		k, v, found := strings.Cut(part, "=")
		if !found || "" == k {
			return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "invalid selector entry %q (expected key=value)", part)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

func filterByLabels(releases []*release.Release, want map[string]string) []*release.Release {
	out := make([]*release.Release, 0, len(releases))
	for _, rel := range releases {
		if matchAllLabels(rel.Labels, want) {
			out = append(out, rel)
		}
	}
	return out
}

func matchAllLabels(have, want map[string]string) bool {
	for k, v := range want {
		if hv, ok := have[k]; !ok || hv != v {
			return false
		}
	}
	return true
}

func latestRevisions(releases []*release.Release) []*release.Release {
	best := make(map[string]*release.Release)
	for _, rel := range releases {
		key := rel.Namespace + "/" + rel.Name
		existing, found := best[key]
		if !found || rel.Revision > existing.Revision {
			best[key] = rel
		}
	}

	result := make([]*release.Release, 0, len(best))
	for _, rel := range best {
		result = append(result, rel)
	}
	return result
}

func filterActiveStatuses(releases []*release.Release) []*release.Release {
	result := make([]*release.Release, 0, len(releases))
	for _, rel := range releases {
		if release.StatusSuperseded == rel.Status || release.StatusFailed == rel.Status {
			continue
		}
		result = append(result, rel)
	}
	return result
}

func filterByRegex(releases []*release.Release, pattern string) ([]*release.Release, error) {
	re, err := regexp.Compile(pattern)
	if nil != err {
		return nil, keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "invalid filter regex: %s", pattern)
	}

	result := make([]*release.Release, 0, len(releases))
	for _, rel := range releases {
		if re.MatchString(rel.Name) {
			result = append(result, rel)
		}
	}
	return result, nil
}

func sortReleases(releases []*release.Release, sortBy string) {
	switch strings.ToLower(sortBy) {
	case "date":
		sort.Slice(releases, func(i, j int) bool {
			return releases[i].Info.LastDeployed.After(releases[j].Info.LastDeployed)
		})
	case "revision":
		sort.Slice(releases, func(i, j int) bool {
			return releases[i].Revision > releases[j].Revision
		})
	default:
		sort.Slice(releases, func(i, j int) bool {
			return releases[i].Name < releases[j].Name
		})
	}
}

func outputReleases(cmd *cobra.Command, releases []*release.Release, output string) error {
	if "json" == output {
		return outputReleasesJSON(cmd, releases)
	}
	if "yaml" == output {
		return outputReleasesYAML(cmd, releases)
	}
	return outputReleasesTable(cmd, releases)
}

func outputReleasesTable(cmd *cobra.Command, releases []*release.Release) error {
	headers := []string{"NAME", "NAMESPACE", "REVISION", "STATUS", "PACKAGE", "VERSION", "UPDATED"}
	rows := make([][]string, 0, len(releases))

	for _, rel := range releases {
		rows = append(rows, []string{
			rel.Name,
			rel.Namespace,
			fmt.Sprintf("%d", rel.Revision),
			string(rel.Status),
			rel.Package.Name,
			rel.Package.Version,
			rel.Info.LastDeployed.Format("2006-01-02 15:04:05"),
		})
	}

	fmt.Fprint(cmd.OutOrStdout(), FormatTable(headers, rows))
	return nil
}

func outputReleasesJSON(cmd *cobra.Command, releases []*release.Release) error {
	out, err := FormatJSON(releases)
	if nil != err {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

func outputReleasesYAML(cmd *cobra.Command, releases []*release.Release) error {
	out, err := FormatYAML(releases)
	if nil != err {
		return err
	}
	fmt.Fprint(cmd.OutOrStdout(), out)
	return nil
}

func validateOutputFormat(output string) error {
	valid := map[string]bool{"table": true, "json": true, "yaml": true}
	if !valid[output] {
		return keramoserr.NewErrorf(keramoserr.ErrCLIValidation, "invalid output format %q, must be table, json, or yaml", output)
	}
	return nil
}
