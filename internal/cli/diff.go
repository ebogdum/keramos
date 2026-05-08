package cli

import (
	"fmt"
	"strings"

	"github.com/ebogdum/keramos/internal/diff"
	"github.com/ebogdum/keramos/internal/engine"
	"github.com/ebogdum/keramos/internal/kube"
	"github.com/ebogdum/keramos/internal/layer"
	"github.com/ebogdum/keramos/internal/release"
	"github.com/ebogdum/keramos/internal/values"
	"github.com/spf13/cobra"
)

func newDiffCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		setStrings []string
		setFiles   []string
		setJSON    []string
		profile    string
		revision   int
		noColor    bool
		smart      bool
		filters    diff.Filters
	)

	cmd := &cobra.Command{
		Use:   "diff <release-name> <package-path>",
		Short: "Show what would change on upgrade",
		Long: `Compare current release manifests with what would be rendered from the given package.

Smart mode (default) groups changes by Kubernetes resource and suppresses
noise classes (status, managed fields, server-set defaults, secret
rotation values). Each noise class can be re-enabled with its own --show-* flag.

Use --smart=false to fall back to a raw line-level unified diff.`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiff(cmd, args[0], args[1], valueFiles, sets, setStrings, setFiles, setJSON, profile, revision, noColor, smart, filters)
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file overrides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "set key=value overrides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "set key=value forcing string interpretation (repeatable)")
	cmd.Flags().StringArrayVar(&setFiles, "set-file", nil, "set key=path; value read from path (repeatable)")
	cmd.Flags().StringArrayVar(&setJSON, "set-json", nil, "set key=<json>; value parsed as JSON (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile name to apply")
	cmd.Flags().IntVar(&revision, "revision", 0, "compare against a specific revision")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored diff output")

	cmd.Flags().BoolVar(&smart, "smart", true, "use Kubernetes-aware structured diff")
	cmd.Flags().BoolVar(&filters.ShowStatus, "show-status", false, "include changes under .status")
	cmd.Flags().BoolVar(&filters.ShowManagedFields, "show-managed-fields", false, "include metadata.managedFields")
	cmd.Flags().BoolVar(&filters.ShowGeneration, "show-generation", false, "include resourceVersion/uid/generation/creationTimestamp")
	cmd.Flags().BoolVar(&filters.ShowDefaultedFields, "show-defaulted-fields", false, "include server-side defaults (clusterIP, port protocol, etc.)")
	cmd.Flags().BoolVar(&filters.ShowAnnotations, "show-annotations", false, "include metadata.annotations")
	cmd.Flags().BoolVar(&filters.ShowLabels, "show-labels", false, "include metadata.labels")
	cmd.Flags().BoolVar(&filters.ShowImagePullPolicy, "show-image-pull-policy", false, "include containers[].imagePullPolicy")
	cmd.Flags().BoolVar(&filters.ShowFinalizers, "show-finalizers", false, "include metadata.finalizers")
	cmd.Flags().BoolVar(&filters.ShowOwnerRefs, "show-owner-refs", false, "include metadata.ownerReferences")
	cmd.Flags().BoolVar(&filters.ShowSecretRotation, "show-secret-rotation", false, "include rotated Secret data values")

	return cmd
}

func runDiff(cmd *cobra.Command, releaseName, packagePath string, valueFiles, sets, setStrings, setFiles, setJSON []string, profile string, revision int, noColor bool, smart bool, filters diff.Filters) error {
	// Step 1: Get current release manifest
	client, err := kube.NewClient(kubeconfig, kubeContext, namespace)
	if nil != err {
		return err
	}

	storage := release.NewSecretStorage(client.Clientset(), client.Namespace())

	var current *release.Release
	if 0 < revision {
		current, err = storage.Get(releaseName, revision)
	} else {
		current, err = storage.Last(releaseName)
	}
	if nil != err {
		return err
	}

	// Step 2: Render new package
	resolved, err := layer.Resolve(packagePath, profile)
	if nil != err {
		return err
	}

	mergedValues, err := values.ResolveAll(map[string]any(resolved.Values), valueFiles, sets, setStrings, setFiles, setJSON)
	if nil != err {
		return err
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
			"namespace": current.Namespace,
			"revision":  current.Revision + 1,
			"isUpgrade": true,
			"isInstall": false,
		},
		Capabilities: map[string]any{},
		Files:        resolved.Files,
	}

	eng := engine.New()
	newManifest, err := eng.Render(resolved.Templates, resolved.Partials, ctx)
	if nil != err {
		return err
	}

	// Step 3: Compute diff (smart-by-default, raw on --smart=false).
	if smart {
		changes, dErr := diff.Compute(current.Manifest, newManifest, filters)
		if nil != dErr {
			return dErr
		}
		if 0 == len(changes) {
			fmt.Fprintln(cmd.OutOrStdout(), "No meaningful changes (noise filtered). Use --show-* flags to see filtered classes, or --smart=false for raw line diff.")
			return nil
		}
		fmt.Fprint(cmd.OutOrStdout(), diff.FormatHuman(changes, !noColor))
		return nil
	}

	diffOutput := UnifiedDiff(current.Manifest, newManifest, "current", "proposed")
	if "" == diffOutput {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes detected.")
		return nil
	}
	if noColor {
		fmt.Fprint(cmd.OutOrStdout(), diffOutput)
		return nil
	}
	fmt.Fprint(cmd.OutOrStdout(), ColorizeDiff(diffOutput))
	return nil
}

// UnifiedDiff produces a unified diff between two strings.
func UnifiedDiff(a, b, labelA, labelB string) string {
	linesA := splitLines(a)
	linesB := splitLines(b)

	// Use a simple LCS-based diff
	edits := computeEdits(linesA, linesB)
	if 0 == len(edits) {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- %s\n", labelA))
	out.WriteString(fmt.Sprintf("+++ %s\n", labelB))

	// Group edits into hunks
	hunks := buildHunks(edits, len(linesA), len(linesB))
	for _, hunk := range hunks {
		out.WriteString(hunk)
	}

	return out.String()
}

// ColorizeDiff adds ANSI color codes to a unified diff.
func ColorizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var out strings.Builder

	colorLookup := map[byte]string{
		'-': "\033[31m", // red
		'+': "\033[32m", // green
		'@': "\033[36m", // cyan
	}
	reset := "\033[0m"

	for _, line := range lines {
		if 0 == len(line) {
			out.WriteString("\n")
			continue
		}
		color, hasColor := colorLookup[line[0]]
		if hasColor {
			out.WriteString(color)
			out.WriteString(line)
			out.WriteString(reset)
		} else {
			out.WriteString(line)
		}
		out.WriteString("\n")
	}

	return out.String()
}

type editOp int

const (
	editEqual  editOp = 0
	editDelete editOp = 1
	editInsert editOp = 2
)

type edit struct {
	op   editOp
	line string
}

func splitLines(s string) []string {
	if "" == s {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty line from trailing newline
	if 0 < len(lines) && "" == lines[len(lines)-1] {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func computeEdits(a, b []string) []edit {
	lenA := len(a)
	lenB := len(b)

	// Build LCS table
	dp := make([][]int, lenA+1)
	for i := range dp {
		dp[i] = make([]int, lenB+1)
	}

	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce edits
	edits := make([]edit, 0, lenA+lenB)
	i, j := lenA, lenB
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			edits = append(edits, edit{op: editEqual, line: a[i-1]})
			i--
			j--
		} else if j > 0 && (0 == i || dp[i][j-1] >= dp[i-1][j]) {
			edits = append(edits, edit{op: editInsert, line: b[j-1]})
			j--
		} else {
			edits = append(edits, edit{op: editDelete, line: a[i-1]})
			i--
		}
	}

	// Reverse
	for left, right := 0, len(edits)-1; left < right; left, right = left+1, right-1 {
		edits[left], edits[right] = edits[right], edits[left]
	}

	// Check if there are any actual changes
	hasChanges := false
	for _, e := range edits {
		if editEqual != e.op {
			hasChanges = true
			break
		}
	}
	if !hasChanges {
		return nil
	}

	return edits
}

func buildHunks(edits []edit, _ int, _ int) []string {
	contextLines := 3
	hunks := make([]string, 0)
	editLen := len(edits)

	// Track line numbers
	lineA := 1
	lineB := 1

	i := 0
	for i < editLen {
		// Find next change
		for i < editLen && editEqual == edits[i].op {
			lineA++
			lineB++
			i++
		}
		if i >= editLen {
			break
		}

		// Start of a hunk - include context before
		hunkStart := i
		hunkLineA := lineA
		hunkLineB := lineB

		contextBefore := make([]edit, 0, contextLines)
		backtrack := i - 1
		for len(contextBefore) < contextLines && backtrack >= 0 {
			contextBefore = append([]edit{edits[backtrack]}, contextBefore...)
			hunkLineA--
			hunkLineB--
			backtrack--
		}

		// Collect changes and context
		hunkEdits := make([]edit, 0)
		hunkEdits = append(hunkEdits, contextBefore...)

		for i < editLen {
			if editEqual == edits[i].op {
				// Check if this is a gap between changes
				contextCount := 0
				j := i
				for j < editLen && editEqual == edits[j].op {
					contextCount++
					j++
				}
				if j >= editLen || contextCount > 2*contextLines {
					// End of hunk - add trailing context
					trailing := contextCount
					if trailing > contextLines {
						trailing = contextLines
					}
					for k := 0; k < trailing; k++ {
						hunkEdits = append(hunkEdits, edits[i+k])
					}
					i += contextCount
					lineA += contextCount
					lineB += contextCount
					break
				}
				// Include the gap
				for k := 0; k < contextCount; k++ {
					hunkEdits = append(hunkEdits, edits[i+k])
				}
				i += contextCount
				lineA += contextCount
				lineB += contextCount
				continue
			}

			hunkEdits = append(hunkEdits, edits[i])
			if editDelete == edits[i].op {
				lineA++
			} else {
				lineB++
			}
			i++
		}

		_ = hunkStart

		// Count lines in hunk for header
		countA := 0
		countB := 0
		for _, e := range hunkEdits {
			if editInsert != e.op {
				countA++
			}
			if editDelete != e.op {
				countB++
			}
		}

		var hunk strings.Builder
		hunk.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", hunkLineA, countA, hunkLineB, countB))

		for _, e := range hunkEdits {
			switch e.op {
			case editEqual:
				hunk.WriteString(" " + e.line + "\n")
			case editDelete:
				hunk.WriteString("-" + e.line + "\n")
			case editInsert:
				hunk.WriteString("+" + e.line + "\n")
			}
		}

		hunks = append(hunks, hunk.String())
	}

	return hunks
}
