package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ebogdum/keramos/v2/internal/diff"
	"github.com/ebogdum/keramos/v2/internal/engine"
	keramoserr "github.com/ebogdum/keramos/v2/internal/errors"
	"github.com/ebogdum/keramos/v2/internal/layer"
	"github.com/ebogdum/keramos/v2/internal/values"
	"github.com/spf13/cobra"
)

// diffSide describes how one side of a comparison is produced, for labelling.
type diffSide struct {
	manifest string
	label    string
}

func newDiffCommand() *cobra.Command {
	var (
		valueFiles []string
		sets       []string
		setStrings []string
		profile    string
		fromValues []string
		toValues   []string
		fromSets   []string
		toSets     []string
		fromProf   string
		toProf     string
		fromRef    string
		toRef      string
		noColor    bool
		smart      bool
	)

	cmd := &cobra.Command{
		Use:   "diff <a> [b]",
		Short: "Compare two packages, manifests, value sets, or git revisions (files only)",
		Long: `Diff is purely file-oriented — it never reads cluster or release state.
(To compare a package against the recorded state use 'keramos plan'; against the
live cluster use 'keramos drift'.) It renders and compares local inputs.

Four modes, chosen from the arguments:

  1. Two package directories — render each and diff:
       keramos diff ./chart-v1 ./chart-v2

  2. Two rendered manifest files — diff them directly, no rendering:
       keramos diff old.yaml new.yaml

  3. One package, two value sets — render it both ways and diff:
       keramos diff ./chart --from-values staging.yaml --to-values prod.yaml
       keramos diff ./chart --to-set replicas=5

  4. One package, two git revisions — render each revision and diff:
       keramos diff ./chart --from-ref v1.2.0 --to-ref HEAD

Shared -f/--set/--profile apply to BOTH sides (useful in mode 1). The
--from-*/--to-* flags set each side independently (mode 3). Output is a smart
per-resource change preview; --smart=false gives a raw line-level unified diff.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, to, err := resolveDiffSides(args, diffSideOpts{
				valueFiles: valueFiles, sets: sets, setStrings: setStrings, profile: profile,
				fromValues: fromValues, toValues: toValues, fromSets: fromSets, toSets: toSets,
				fromProf: fromProf, toProf: toProf, fromRef: fromRef, toRef: toRef,
			})
			if nil != err {
				return err
			}
			return writeDiff(cmd, from, to, smart, !noColor)
		},
	}

	cmd.Flags().StringArrayVarP(&valueFiles, "values", "f", nil, "values file applied to BOTH sides (repeatable)")
	cmd.Flags().StringArrayVar(&sets, "set", nil, "key=value applied to BOTH sides (repeatable)")
	cmd.Flags().StringArrayVar(&setStrings, "set-string", nil, "key=value (string) applied to BOTH sides (repeatable)")
	cmd.Flags().StringVar(&profile, "profile", "", "profile applied to BOTH sides")
	cmd.Flags().StringArrayVar(&fromValues, "from-values", nil, "values file for the FROM side only (mode 3)")
	cmd.Flags().StringArrayVar(&toValues, "to-values", nil, "values file for the TO side only (mode 3)")
	cmd.Flags().StringArrayVar(&fromSets, "from-set", nil, "key=value for the FROM side only (mode 3)")
	cmd.Flags().StringArrayVar(&toSets, "to-set", nil, "key=value for the TO side only (mode 3)")
	cmd.Flags().StringVar(&fromProf, "from-profile", "", "profile for the FROM side only (mode 3)")
	cmd.Flags().StringVar(&toProf, "to-profile", "", "profile for the TO side only (mode 3)")
	cmd.Flags().StringVar(&fromRef, "from-ref", "", "git revision for the FROM side (mode 4)")
	cmd.Flags().StringVar(&toRef, "to-ref", "", "git revision for the TO side (mode 4, default: working tree)")
	cmd.Flags().BoolVar(&smart, "smart", true, "smart per-resource diff; --smart=false for raw unified diff")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")
	return cmd
}

type diffSideOpts struct {
	valueFiles, sets, setStrings []string
	profile                      string
	fromValues, toValues         []string
	fromSets, toSets             []string
	fromProf, toProf             string
	fromRef, toRef               string
}

// resolveDiffSides interprets the arguments and flags into two labelled
// manifests to compare, selecting one of the four diff modes.
func resolveDiffSides(args []string, o diffSideOpts) (diffSide, diffSide, error) {
	hasFromTo := 0 < len(o.fromValues) || 0 < len(o.toValues) || 0 < len(o.fromSets) ||
		0 < len(o.toSets) || "" != o.fromProf || "" != o.toProf
	hasRef := "" != o.fromRef || "" != o.toRef

	if 2 == len(args) {
		if hasRef || hasFromTo {
			return diffSide{}, diffSide{}, keramoserr.NewError(keramoserr.ErrCLIValidation,
				"--from-*/--to-*/--*-ref flags take a single package argument; you passed two")
		}
		aDir, bDir := isDir(args[0]), isDir(args[1])
		if aDir && bDir {
			// Mode 1: two package directories.
			fm, err := renderPkgManifest(args[0], o.profile, o.valueFiles, o.sets, o.setStrings)
			if nil != err {
				return diffSide{}, diffSide{}, err
			}
			tm, err := renderPkgManifest(args[1], o.profile, o.valueFiles, o.sets, o.setStrings)
			if nil != err {
				return diffSide{}, diffSide{}, err
			}
			return diffSide{fm, args[0]}, diffSide{tm, args[1]}, nil
		}
		if isFile(args[0]) && isFile(args[1]) {
			// Mode 2: two rendered manifest files.
			fm, err := os.ReadFile(args[0])
			if nil != err {
				return diffSide{}, diffSide{}, keramoserr.WrapError(keramoserr.ErrCLIValidation, "read manifest", err)
			}
			tm, err := os.ReadFile(args[1])
			if nil != err {
				return diffSide{}, diffSide{}, keramoserr.WrapError(keramoserr.ErrCLIValidation, "read manifest", err)
			}
			return diffSide{string(fm), args[0]}, diffSide{string(tm), args[1]}, nil
		}
		return diffSide{}, diffSide{}, keramoserr.NewError(keramoserr.ErrCLIValidation,
			"both arguments must be package directories, or both rendered manifest files")
	}

	// Single positional.
	dir := args[0]
	if !isDir(dir) {
		return diffSide{}, diffSide{}, keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"%q is not a package directory; give a second argument to diff two files", dir)
	}
	if hasRef {
		// Mode 4: two git revisions of the same package.
		fromRef := o.fromRef
		if "" == fromRef {
			return diffSide{}, diffSide{}, keramoserr.NewError(keramoserr.ErrCLIValidation,
				"--from-ref is required for a git-revision diff")
		}
		fm, err := renderPkgAtRef(dir, fromRef, o.profile, o.valueFiles, o.sets, o.setStrings)
		if nil != err {
			return diffSide{}, diffSide{}, err
		}
		var tm string
		toLabel := "working tree"
		if "" != o.toRef {
			tm, err = renderPkgAtRef(dir, o.toRef, o.profile, o.valueFiles, o.sets, o.setStrings)
			toLabel = o.toRef
		} else {
			tm, err = renderPkgManifest(dir, o.profile, o.valueFiles, o.sets, o.setStrings)
		}
		if nil != err {
			return diffSide{}, diffSide{}, err
		}
		return diffSide{fm, "@" + fromRef}, diffSide{tm, "@" + toLabel}, nil
	}
	if hasFromTo {
		// Mode 3: one package rendered under two value sets.
		fm, err := renderPkgManifest(dir, firstNonEmpty(o.fromProf, o.profile),
			append(append([]string{}, o.valueFiles...), o.fromValues...),
			append(append([]string{}, o.sets...), o.fromSets...), o.setStrings)
		if nil != err {
			return diffSide{}, diffSide{}, err
		}
		tm, err := renderPkgManifest(dir, firstNonEmpty(o.toProf, o.profile),
			append(append([]string{}, o.valueFiles...), o.toValues...),
			append(append([]string{}, o.sets...), o.toSets...), o.setStrings)
		if nil != err {
			return diffSide{}, diffSide{}, err
		}
		return diffSide{fm, "from"}, diffSide{tm, "to"}, nil
	}
	return diffSide{}, diffSide{}, keramoserr.NewError(keramoserr.ErrCLIValidation,
		"nothing to compare against: pass a second package/manifest, or --from-values/--to-values, or --from-ref/--to-ref")
}

// writeDiff renders the comparison, smart by default with a raw-unified fallback.
func writeDiff(cmd *cobra.Command, from, to diffSide, smart, color bool) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "diff: %s → %s\n\n", from.label, to.label)
	if smart {
		changes, err := diff.Compute(from.manifest, to.manifest, allShownFilters())
		if nil != err {
			return keramoserr.WrapError(keramoserr.ErrInternal, "compute diff", err)
		}
		if 0 == len(changes) {
			fmt.Fprintln(w, "No differences.")
			return nil
		}
		fmt.Fprint(w, formatPlanChanges(changes, color, nil, ""))
		fmt.Fprint(w, changeSummary(changes))
		return nil
	}
	out := UnifiedDiff(from.manifest, to.manifest, from.label, to.label)
	if "" == out {
		fmt.Fprintln(w, "No differences.")
		return nil
	}
	if color {
		out = ColorizeDiff(out)
	}
	fmt.Fprint(w, out)
	return nil
}

// renderPkgManifest resolves and renders a package directory to a manifest,
// with no cluster access — the shared engine path used by plan/template.
func renderPkgManifest(dir, profile string, valueFiles, sets, setStrings []string) (string, error) {
	resolved, err := layer.Resolve(dir, profile)
	if nil != err {
		return "", err
	}
	merged, err := values.ResolveAll(map[string]any(resolved.Values), valueFiles, sets, setStrings, nil, nil)
	if nil != err {
		return "", err
	}
	name := ""
	ver := ""
	appVer := ""
	if nil != resolved.Metadata {
		name, ver, appVer = resolved.Metadata.Name, resolved.Metadata.Version, resolved.Metadata.AppVersion
	}
	ctx := &engine.RenderContext{
		Values: merged,
		Package: map[string]any{
			"name": name, "version": ver, "appVersion": appVer,
		},
		Release: map[string]any{
			"name": name, "namespace": namespace, "revision": 1,
			"isInstall": true, "isUpgrade": false,
		},
		Capabilities: map[string]any{},
		Files:        resolved.Files,
	}
	return engine.New().Render(resolved.Templates, resolved.Partials, ctx)
}

// renderPkgAtRef checks out the package directory at a git revision into a temp
// worktree and renders it there. Requires the directory to be inside a git repo.
func renderPkgAtRef(dir, ref, profile string, valueFiles, sets, setStrings []string) (string, error) {
	// A ref beginning with '-' would be parsed by `git archive` as an option
	// (e.g. --output, --remote) rather than a tree-ish, since it sits before
	// the `--` separator. Reject it.
	if strings.HasPrefix(ref, "-") {
		return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"git revision %q must not begin with '-'", ref)
	}
	root, relErr := gitRepoRoot(dir)
	if nil != relErr {
		return "", relErr
	}
	rel, err := filepath.Rel(root, mustAbs(dir))
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrCLIValidation, "locate package within repo", err)
	}
	tmp, err := os.MkdirTemp("", "keramos-diff-ref-")
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "temp dir", err)
	}
	defer os.RemoveAll(tmp)

	// `git archive <ref> -- <rel>` emits a tar of that subtree; pipe into tar -x.
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", ref, "--", rel)
	untar := exec.Command("tar", "-x", "-C", tmp)
	pipe, err := archive.StdoutPipe()
	if nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "git archive pipe", err)
	}
	untar.Stdin = pipe
	var aerr strings.Builder
	archive.Stderr = &aerr
	if err := untar.Start(); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "start tar", err)
	}
	if err := archive.Run(); nil != err {
		return "", keramoserr.WrapErrorf(keramoserr.ErrCLIValidation, err,
			"git archive %s failed: %s", ref, strings.TrimSpace(aerr.String()))
	}
	if err := untar.Wait(); nil != err {
		return "", keramoserr.WrapError(keramoserr.ErrInternal, "extract archive", err)
	}
	return renderPkgManifest(filepath.Join(tmp, rel), profile, valueFiles, sets, setStrings)
}

func gitRepoRoot(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if nil != err {
		return "", keramoserr.NewErrorf(keramoserr.ErrCLIValidation,
			"%q is not inside a git repository; --from-ref/--to-ref need git", dir)
	}
	return strings.TrimSpace(string(out)), nil
}

func mustAbs(p string) string {
	a, err := filepath.Abs(p)
	if nil != err {
		return p
	}
	return a
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return nil == err && fi.IsDir()
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	return nil == err && !fi.IsDir()
}

func firstNonEmpty(a, b string) string {
	if "" != a {
		return a
	}
	return b
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
